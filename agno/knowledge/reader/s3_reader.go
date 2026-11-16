package reader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/devalexandre/agno-golang/agno/document"
)

// S3API is the subset of *s3.Client used by S3Reader.
type S3API interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// S3Reader reads objects from Amazon S3 or any S3-compatible store (MinIO,
// R2, ...) with the AWS SDK v2. Port of agno-python
// knowledge/reader/s3_reader.py: PDFs go through PDFReader, other objects
// through the reader matching their extension (ReaderForExtension) and fall
// back to TextReader.
//
// Sources are "s3://bucket/key" for one object or "s3://bucket/prefix/"
// (trailing slash) for every object under a prefix (up to MaxObjects).
type S3Reader struct {
	BaseReader
	// Client to use; when nil one is built from the default AWS config
	// (env vars, shared config, IAM role) plus Region/Endpoint/UsePathStyle.
	Client       S3API
	Region       string
	Endpoint     string
	UsePathStyle bool
	// MaxObjects bounds prefix reads (default 100).
	MaxObjects int
	// MaxObjectBytes caps one object's size (default 64 MiB).
	MaxObjectBytes int64
}

// NewS3Reader creates an S3 reader.
func NewS3Reader() *S3Reader {
	return &S3Reader{BaseReader: BaseReader{ID: "s3", Extensions: []string{"s3"}}, MaxObjects: 100}
}

// ParseS3URI splits "s3://bucket/key" into bucket and key.
func ParseS3URI(uri string) (bucket, key string, err error) {
	if !strings.HasPrefix(uri, "s3://") {
		return "", "", fmt.Errorf("s3: %q is not an s3:// URI", uri)
	}
	rest := strings.TrimPrefix(uri, "s3://")
	bucket, key, _ = strings.Cut(rest, "/")
	if bucket == "" {
		return "", "", fmt.Errorf("s3: missing bucket in %q", uri)
	}
	return bucket, key, nil
}

func (r *S3Reader) client(ctx context.Context) (S3API, error) {
	if r.Client != nil {
		return r.Client, nil
	}
	var opts []func(*config.LoadOptions) error
	if r.Region != "" {
		opts = append(opts, config.WithRegion(r.Region))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("s3: load AWS config: %w", err)
	}
	c := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if r.Endpoint != "" {
			o.BaseEndpoint = aws.String(r.Endpoint)
		}
		o.UsePathStyle = r.UsePathStyle
	})
	r.Client = c
	return c, nil
}

// Read reads one object or every object under a prefix.
func (r *S3Reader) Read(ctx context.Context, source string) ([]*document.Document, error) {
	bucket, key, err := ParseS3URI(source)
	if err != nil {
		return nil, err
	}
	c, err := r.client(ctx)
	if err != nil {
		return nil, err
	}
	if key != "" && !strings.HasSuffix(key, "/") {
		return r.readObject(ctx, c, bucket, key)
	}
	max := r.MaxObjects
	if max <= 0 {
		max = 100
	}
	var docs []*document.Document
	var token *string
	count := 0
	for count < max {
		out, err := c.ListObjectsV2(ctx, &s3.ListObjectsV2Input{Bucket: aws.String(bucket), Prefix: aws.String(key), ContinuationToken: token})
		if err != nil {
			return nil, fmt.Errorf("s3: list s3://%s/%s: %w", bucket, key, err)
		}
		for _, obj := range out.Contents {
			k := aws.ToString(obj.Key)
			if strings.HasSuffix(k, "/") {
				continue
			}
			if count >= max {
				break
			}
			count++
			d, err := r.readObject(ctx, c, bucket, k)
			if err != nil {
				return nil, err
			}
			docs = append(docs, d...)
		}
		if !aws.ToBool(out.IsTruncated) || out.NextContinuationToken == nil {
			break
		}
		token = out.NextContinuationToken
	}
	return docs, nil
}

func (r *S3Reader) readObject(ctx context.Context, c S3API, bucket, key string) ([]*document.Document, error) {
	uri := "s3://" + bucket + "/" + key
	out, err := c.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(bucket), Key: aws.String(key)})
	if err != nil {
		return nil, fmt.Errorf("s3: get %s: %w", uri, err)
	}
	defer out.Body.Close()
	limit := r.MaxObjectBytes
	if limit <= 0 {
		limit = defaultMaxBytes
	}
	data, err := io.ReadAll(io.LimitReader(out.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("s3: read %s: %w", uri, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("s3: %s exceeds %d bytes", uri, limit)
	}

	base := path.Base(key)
	ext := strings.ToLower(path.Ext(base))
	inner := ReaderForExtension(ext)
	if inner == nil {
		inner = NewTextReader()
	}
	// Inner readers take a path: stage the object in a temp file keeping its extension.
	f, err := os.CreateTemp("", "agno-s3-*"+ext)
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()
	docs, err := inner.Read(ctx, f.Name())
	if err != nil {
		return nil, fmt.Errorf("s3: %s: %w", uri, err)
	}
	name := strings.ReplaceAll(strings.SplitN(base, ".", 2)[0], " ", "_")
	for _, d := range docs {
		d.Name = name
		d.Source = uri
		d.Metadata["source"] = uri
		d.Metadata["s3_bucket"] = bucket
		d.Metadata["s3_key"] = key
	}
	return docs, nil
}

var _ Reader = (*S3Reader)(nil)
