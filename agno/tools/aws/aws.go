package aws

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/devalexandre/agno-golang/agno/tools/toolkit"
)

// AWSTools provides tools for interacting with AWS services.
type AWSTools struct {
	*toolkit.Toolkit
	s3Client  *s3.Client
	ec2Client *ec2.Client
	region    string
}

// NewAWSTools creates a new AWSTools instance.
func NewAWSTools(region string) *AWSTools {
	tk := toolkit.NewToolkit()
	tk.Name = "aws"
	tk.Description = "Tools for interacting with AWS services like S3 and EC2."

	awsTools := &AWSTools{
		Toolkit: &tk,
		region:  region,
	}

	// Register S3 methods
	awsTools.Register("ListBuckets", "Lists all S3 buckets in the account.", awsTools, awsTools.ListBuckets, ListBucketsParams{})
	awsTools.Register("UploadFile", "Uploads a file to an S3 bucket.", awsTools, awsTools.UploadFile, UploadFileParams{})
	awsTools.Register("DownloadFile", "Downloads a file from an S3 bucket.", awsTools, awsTools.DownloadFile, DownloadFileParams{})
	awsTools.Register("DeleteFile", "Deletes a file from an S3 bucket.", awsTools, awsTools.DeleteFile, DeleteFileParams{})

	// Register EC2 methods
	awsTools.Register("DescribeInstances", "Lists EC2 instances.", awsTools, awsTools.DescribeInstances, DescribeInstancesParams{})
	awsTools.Register("StartInstance", "Starts an EC2 instance.", awsTools, awsTools.StartInstance, InstanceParams{})
	awsTools.Register("StopInstance", "Stops an EC2 instance.", awsTools, awsTools.StopInstance, InstanceParams{})

	return awsTools
}

// Connect initializes the AWS clients.
func (a *AWSTools) Connect(ctx context.Context) error {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(a.region))
	if err != nil {
		return fmt.Errorf("unable to load SDK config, %v", err)
	}

	a.s3Client = s3.NewFromConfig(cfg)
	a.ec2Client = ec2.NewFromConfig(cfg)
	return nil
}

// --- S3 Methods ---

type ListBucketsParams struct{}

func (a *AWSTools) ListBuckets(params ListBucketsParams) (interface{}, error) {
	result, err := a.s3Client.ListBuckets(context.TODO(), &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("unable to list buckets, %v", err)
	}

	var buckets []string
	for _, b := range result.Buckets {
		buckets = append(buckets, aws.ToString(b.Name))
	}

	return buckets, nil
}

type UploadFileParams struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	FilePath string `json:"file_path"`
}

func (a *AWSTools) UploadFile(params UploadFileParams) (string, error) {
	file, err := os.Open(params.FilePath)
	if err != nil {
		return "", fmt.Errorf("unable to open file %v, %v", params.FilePath, err)
	}
	defer file.Close()

	_, err = a.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String(params.Bucket),
		Key:    aws.String(params.Key),
		Body:   file,
	})
	if err != nil {
		return "", fmt.Errorf("unable to upload file to bucket %v, %v", params.Bucket, err)
	}

	return fmt.Sprintf("Successfully uploaded %s to %s/%s", params.FilePath, params.Bucket, params.Key), nil
}

type DownloadFileParams struct {
	Bucket   string `json:"bucket"`
	Key      string `json:"key"`
	FilePath string `json:"file_path"`
}

func (a *AWSTools) DownloadFile(params DownloadFileParams) (string, error) {
	result, err := a.s3Client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket: aws.String(params.Bucket),
		Key:    aws.String(params.Key),
	})
	if err != nil {
		return "", fmt.Errorf("unable to download item from bucket %v, %v", params.Bucket, err)
	}
	defer result.Body.Close()

	file, err := os.Create(params.FilePath)
	if err != nil {
		return "", fmt.Errorf("unable to create file %v, %v", params.FilePath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, result.Body)
	if err != nil {
		return "", fmt.Errorf("unable to save file %v, %v", params.FilePath, err)
	}

	return fmt.Sprintf("Successfully downloaded %s/%s to %s", params.Bucket, params.Key, params.FilePath), nil
}

type DeleteFileParams struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
}

func (a *AWSTools) DeleteFile(params DeleteFileParams) (string, error) {
	_, err := a.s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(params.Bucket),
		Key:    aws.String(params.Key),
	})
	if err != nil {
		return "", fmt.Errorf("unable to delete object %v from bucket %v, %v", params.Key, params.Bucket, err)
	}

	return fmt.Sprintf("Successfully deleted %s from %s", params.Key, params.Bucket), nil
}

type InstanceParams struct {
	InstanceID string `json:"instance_id"`
}

type DescribeInstancesParams struct{}

func (a *AWSTools) DescribeInstances(params DescribeInstancesParams) (interface{}, error) {
	result, err := a.ec2Client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("unable to describe instances, %v", err)
	}

	type instanceInfo struct {
		InstanceID string `json:"instance_id"`
		State      string `json:"state"`
		Type       string `json:"type"`
	}

	var instances []instanceInfo
	for _, reservation := range result.Reservations {
		for _, inst := range reservation.Instances {
			instances = append(instances, instanceInfo{
				InstanceID: aws.ToString(inst.InstanceId),
				State:      string(inst.State.Name),
				Type:       string(inst.InstanceType),
			})
		}
	}

	return instances, nil
}

func (a *AWSTools) StartInstance(params InstanceParams) (string, error) {
	_, err := a.ec2Client.StartInstances(context.TODO(), &ec2.StartInstancesInput{
		InstanceIds: []string{params.InstanceID},
	})
	if err != nil {
		return "", fmt.Errorf("unable to start instance %v, %v", params.InstanceID, err)
	}

	return fmt.Sprintf("Successfully started instance %s", params.InstanceID), nil
}

func (a *AWSTools) StopInstance(params InstanceParams) (string, error) {
	_, err := a.ec2Client.StopInstances(context.TODO(), &ec2.StopInstancesInput{
		InstanceIds: []string{params.InstanceID},
	})
	if err != nil {
		return "", fmt.Errorf("unable to stop instance %v, %v", params.InstanceID, err)
	}

	return fmt.Sprintf("Successfully stopped instance %s", params.InstanceID), nil
}
