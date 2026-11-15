// S3Reader: read one object ("s3://bucket/key") or a whole prefix
// ("s3://bucket/prefix/") from S3 or any S3-compatible store; each object is
// parsed by the reader matching its extension (pdf, xlsx, json, md, ...).
//
// Credentials come from the default AWS chain (AWS_ACCESS_KEY_ID /
// AWS_SECRET_ACCESS_KEY / AWS_REGION, ~/.aws, IAM role). For MinIO/R2 set
// S3_ENDPOINT (e.g. http://localhost:9000; path-style addressing is used).
//
// Usage: go run ./cookbook/knowledge/reader/s3 s3://my-bucket/docs/
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/devalexandre/agno-golang/agno/knowledge/reader"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: s3 s3://bucket/key-or-prefix/")
	}
	r := reader.NewS3Reader()
	r.Region = os.Getenv("AWS_REGION")
	if ep := os.Getenv("S3_ENDPOINT"); ep != "" {
		r.Endpoint = ep
		r.UsePathStyle = true
	}
	r.MaxObjects = 20
	docs, err := r.Read(context.Background(), os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	for _, d := range docs {
		content := d.Content
		if len(content) > 150 {
			content = content[:150] + "..."
		}
		fmt.Printf("== %s (%s)\n%s\n\n", d.Name, d.Source, content)
	}
}
