package azure

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// AzureTools provides tools for interacting with Microsoft Azure services.
type AzureTools struct {
	*toolkit.Toolkit
	subscriptionID string
	blobClient     *azblob.Client
	vmClient       *armcompute.VirtualMachinesClient
}

// NewAzureTools creates a new AzureTools instance.
func NewAzureTools(subscriptionID string) *AzureTools {
	tk := toolkit.NewToolkit()
	tk.Name = "azure"
	tk.Description = "Tools for interacting with Microsoft Azure services like Blob Storage and Virtual Machines."

	azureTools := &AzureTools{
		Toolkit:        &tk,
		subscriptionID: subscriptionID,
	}

	// Register Blob Storage methods
	azureTools.Register("ListContainers", "Lists all blob containers in the account.", azureTools, azureTools.ListContainers, ListContainersParams{})
	azureTools.Register("UploadBlob", "Uploads a file to a blob container.", azureTools, azureTools.UploadBlob, UploadBlobParams{})
	azureTools.Register("DownloadBlob", "Downloads a file from a blob container.", azureTools, azureTools.DownloadBlob, DownloadBlobParams{})
	azureTools.Register("DeleteBlob", "Deletes a file from a blob container.", azureTools, azureTools.DeleteBlob, DeleteBlobParams{})

	// Register Virtual Machine methods
	azureTools.Register("ListVMs", "Lists Azure Virtual Machines.", azureTools, azureTools.ListVMs, ListVMsParams{})
	azureTools.Register("StartVM", "Starts an Azure Virtual Machine.", azureTools, azureTools.StartVM, VMParams{})
	azureTools.Register("StopVM", "Stops an Azure Virtual Machine.", azureTools, azureTools.StopVM, VMParams{})

	return azureTools
}

// Connect initializes the Azure clients.
func (a *AzureTools) Connect(ctx context.Context, accountName string) error {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return fmt.Errorf("failed to obtain a credential: %v", err)
	}

	// Blob Client
	serviceURL := fmt.Sprintf("https://%s.blob.core.windows.net/", accountName)
	blobClient, err := azblob.NewClient(serviceURL, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create blob client: %v", err)
	}
	a.blobClient = blobClient

	// VM Client
	vmClient, err := armcompute.NewVirtualMachinesClient(a.subscriptionID, cred, nil)
	if err != nil {
		return fmt.Errorf("failed to create VM client: %v", err)
	}
	a.vmClient = vmClient

	return nil
}

// --- Blob Storage Methods ---

type ListContainersParams struct{}

func (a *AzureTools) ListContainers(params ListContainersParams) (interface{}, error) {
	var containers []string
	pager := a.blobClient.NewListContainersPager(nil)
	for pager.More() {
		resp, err := pager.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("failed to list containers: %v", err)
		}
		for _, container := range resp.ContainerItems {
			containers = append(containers, *container.Name)
		}
	}
	return containers, nil
}

type UploadBlobParams struct {
	Container string `json:"container"`
	BlobName  string `json:"blob_name"`
	FilePath  string `json:"file_path"`
}

func (a *AzureTools) UploadBlob(params UploadBlobParams) (string, error) {
	file, err := os.Open(params.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	_, err = a.blobClient.UploadFile(context.TODO(), params.Container, params.BlobName, file, nil)
	if err != nil {
		return "", fmt.Errorf("failed to upload blob: %v", err)
	}

	return fmt.Sprintf("Successfully uploaded %s to container %s as %s", params.FilePath, params.Container, params.BlobName), nil
}

type DownloadBlobParams struct {
	Container string `json:"container"`
	BlobName  string `json:"blob_name"`
	FilePath  string `json:"file_path"`
}

func (a *AzureTools) DownloadBlob(params DownloadBlobParams) (string, error) {
	file, err := os.Create(params.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	_, err = a.blobClient.DownloadFile(context.TODO(), params.Container, params.BlobName, file, nil)
	if err != nil {
		return "", fmt.Errorf("failed to download blob: %v", err)
	}

	return fmt.Sprintf("Successfully downloaded blob %s from container %s to %s", params.BlobName, params.Container, params.FilePath), nil
}

type DeleteBlobParams struct {
	Container string `json:"container"`
	BlobName  string `json:"blob_name"`
}

func (a *AzureTools) DeleteBlob(params DeleteBlobParams) (string, error) {
	_, err := a.blobClient.DeleteBlob(context.TODO(), params.Container, params.BlobName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to delete blob: %v", err)
	}
	return fmt.Sprintf("Successfully deleted blob %s from container %s", params.BlobName, params.Container), nil
}

// --- VM Methods ---

type ListVMsParams struct {
	ResourceGroup string `json:"resource_group"`
}

func (a *AzureTools) ListVMs(params ListVMsParams) (interface{}, error) {
	var vms []map[string]string
	pager := a.vmClient.NewListPager(params.ResourceGroup, nil)
	for pager.More() {
		resp, err := pager.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("failed to list VMs: %v", err)
		}
		for _, vm := range resp.Value {
			vms = append(vms, map[string]string{
				"name":     *vm.Name,
				"location": *vm.Location,
			})
		}
	}
	return vms, nil
}

type VMParams struct {
	ResourceGroup string `json:"resource_group"`
	VMName        string `json:"vm_name"`
}

func (a *AzureTools) StartVM(params VMParams) (string, error) {
	poller, err := a.vmClient.BeginStart(context.TODO(), params.ResourceGroup, params.VMName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to start VM: %v", err)
	}
	_, err = poller.PollUntilDone(context.TODO(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to wait for VM start: %v", err)
	}
	return fmt.Sprintf("Successfully started VM %s in resource group %s", params.VMName, params.ResourceGroup), nil
}

func (a *AzureTools) StopVM(params VMParams) (string, error) {
	poller, err := a.vmClient.BeginDeallocate(context.TODO(), params.ResourceGroup, params.VMName, nil)
	if err != nil {
		return "", fmt.Errorf("failed to stop VM: %v", err)
	}
	_, err = poller.PollUntilDone(context.TODO(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to wait for VM stop: %v", err)
	}
	return fmt.Sprintf("Successfully stopped VM %s in resource group %s", params.VMName, params.ResourceGroup), nil
}
