package gcp

import (
	"context"
	"fmt"
	"io"
	"os"

	"cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"cloud.google.com/go/storage"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GCPTools provides tools for interacting with Google Cloud Platform services.
type GCPTools struct {
	*toolkit.Toolkit
	projectID     string
	storageClient *storage.Client
	computeClient *compute.InstancesClient
	credsPath     string
}

// NewGCPTools creates a new GCPTools instance.
func NewGCPTools(projectID string, credsPath string) *GCPTools {
	tk := toolkit.NewToolkit()
	tk.Name = "gcp"
	tk.Description = "Tools for interacting with Google Cloud Platform services like GCS and Compute Engine."

	gcpTools := &GCPTools{
		Toolkit:   &tk,
		projectID: projectID,
		credsPath: credsPath,
	}

	// Register GCS methods
	gcpTools.Register("ListBuckets", "Lists all GCS buckets in the project.", gcpTools, gcpTools.ListBuckets, ListBucketsParams{})
	gcpTools.Register("UploadFile", "Uploads a file to a GCS bucket.", gcpTools, gcpTools.UploadFile, UploadFileParams{})
	gcpTools.Register("DownloadFile", "Downloads a file from a GCS bucket.", gcpTools, gcpTools.DownloadFile, DownloadFileParams{})
	gcpTools.Register("DeleteFile", "Deletes a file from a GCS bucket.", gcpTools, gcpTools.DeleteFile, DeleteFileParams{})

	// Register Compute Engine methods
	gcpTools.Register("ListInstances", "Lists Compute Engine instances.", gcpTools, gcpTools.ListInstances, ListInstancesParams{})
	gcpTools.Register("StartInstance", "Starts a Compute Engine instance.", gcpTools, gcpTools.StartInstance, InstanceParams{})
	gcpTools.Register("StopInstance", "Stops a Compute Engine instance.", gcpTools, gcpTools.StopInstance, InstanceParams{})

	return gcpTools
}

// Connect initializes the GCP clients.
func (g *GCPTools) Connect(ctx context.Context) error {
	var opts []option.ClientOption
	if g.credsPath != "" {
		opts = append(opts, option.WithCredentialsFile(g.credsPath))
	}

	storageClient, err := storage.NewClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %v", err)
	}
	g.storageClient = storageClient

	computeClient, err := compute.NewInstancesRESTClient(ctx, opts...)
	if err != nil {
		return fmt.Errorf("failed to create compute client: %v", err)
	}
	g.computeClient = computeClient

	return nil
}

// Close closes the GCP clients.
func (g *GCPTools) Close() error {
	if g.storageClient != nil {
		g.storageClient.Close()
	}
	if g.computeClient != nil {
		g.computeClient.Close()
	}
	return nil
}

// --- GCS Methods ---

type ListBucketsParams struct{}

func (g *GCPTools) ListBuckets(params ListBucketsParams) (interface{}, error) {
	var buckets []string
	it := g.storageClient.Buckets(context.TODO(), g.projectID)
	for {
		battrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list buckets: %v", err)
		}
		buckets = append(buckets, battrs.Name)
	}
	return buckets, nil
}

type UploadFileParams struct {
	Bucket   string `json:"bucket"`
	Object   string `json:"object"`
	FilePath string `json:"file_path"`
}

func (g *GCPTools) UploadFile(params UploadFileParams) (string, error) {
	f, err := os.Open(params.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()

	wc := g.storageClient.Bucket(params.Bucket).Object(params.Object).NewWriter(context.TODO())
	if _, err = io.Copy(wc, f); err != nil {
		return "", fmt.Errorf("failed to copy file to GCS: %v", err)
	}
	if err := wc.Close(); err != nil {
		return "", fmt.Errorf("failed to close GCS writer: %v", err)
	}

	return fmt.Sprintf("Successfully uploaded %s to gs://%s/%s", params.FilePath, params.Bucket, params.Object), nil
}

type DownloadFileParams struct {
	Bucket   string `json:"bucket"`
	Object   string `json:"object"`
	FilePath string `json:"file_path"`
}

func (g *GCPTools) DownloadFile(params DownloadFileParams) (string, error) {
	rc, err := g.storageClient.Bucket(params.Bucket).Object(params.Object).NewReader(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to create GCS reader: %v", err)
	}
	defer rc.Close()

	f, err := os.Create(params.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create local file: %v", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, rc); err != nil {
		return "", fmt.Errorf("failed to copy from GCS to local file: %v", err)
	}

	return fmt.Sprintf("Successfully downloaded gs://%s/%s to %s", params.Bucket, params.Object, params.FilePath), nil
}

type DeleteFileParams struct {
	Bucket string `json:"bucket"`
	Object string `json:"object"`
}

func (g *GCPTools) DeleteFile(params DeleteFileParams) (string, error) {
	err := g.storageClient.Bucket(params.Bucket).Object(params.Object).Delete(context.TODO())
	if err != nil {
		return "", fmt.Errorf("failed to delete object: %v", err)
	}
	return fmt.Sprintf("Successfully deleted gs://%s/%s", params.Bucket, params.Object), nil
}

// --- Compute Engine Methods ---

type ListInstancesParams struct {
	Zone string `json:"zone"`
}

func (g *GCPTools) ListInstances(params ListInstancesParams) (interface{}, error) {
	var instances []map[string]string
	it := g.computeClient.List(context.TODO(), &computepb.ListInstancesRequest{
		Project: g.projectID,
		Zone:    params.Zone,
	})
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list instances: %v", err)
		}
		instances = append(instances, map[string]string{
			"name":   resp.GetName(),
			"status": resp.GetStatus(),
		})
	}
	return instances, nil
}

type InstanceParams struct {
	Zone     string `json:"zone"`
	Instance string `json:"instance"`
}

func (g *GCPTools) StartInstance(params InstanceParams) (string, error) {
	_, err := g.computeClient.Start(context.TODO(), &computepb.StartInstanceRequest{
		Project:  g.projectID,
		Zone:     params.Zone,
		Instance: params.Instance,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start instance: %v", err)
	}
	return fmt.Sprintf("Successfully started instance %s in zone %s", params.Instance, params.Zone), nil
}

func (g *GCPTools) StopInstance(params InstanceParams) (string, error) {
	_, err := g.computeClient.Stop(context.TODO(), &computepb.StopInstanceRequest{
		Project:  g.projectID,
		Zone:     params.Zone,
		Instance: params.Instance,
	})
	if err != nil {
		return "", fmt.Errorf("failed to stop instance: %v", err)
	}
	return fmt.Sprintf("Successfully stopped instance %s in zone %s", params.Instance, params.Zone), nil
}
