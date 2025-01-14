package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/dhis2-sre/im-manager/pkg/instance"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	ctx := context.Background()

	minioEndpoint := "localhost:9000"
	minioAccessKey := "dhisdhis"
	minioSecretKey := "dhisdhis"
	minioBucket := "dhis2"
	minioUseSSL := false

	s3Region := "eu-west-1"
	s3Bucket := "im-databases-feature"

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	minioClient, err := setupMinioClient(minioAccessKey, minioSecretKey, minioEndpoint, minioUseSSL)
	if err != nil {
		log.Fatalf("minio client setup: %v", err)
	}

	s3Client, err := setupS3Client(ctx, s3Region)
	if err != nil {
		log.Fatalf("s3 client setup: %v", err)
	}

	service := instance.NewBackupService(logger, minioClient, s3Client)

	timestamp := time.Now().Format(time.RFC3339)
	key := fmt.Sprintf("backup-%s.tar.gz", timestamp)
	if err := service.PerformBackup(ctx, minioBucket, s3Bucket, key); err != nil {
		log.Fatalf("Backup failed: %v", err)
	}
}

func setupS3Client(ctx context.Context, region string) (*s3.Client, error) {
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %v", err)
	}
	return s3.NewFromConfig(awsConfig), nil
}

func setupMinioClient(accessKey, secretKey, endpoint string, useSSL bool) (*minio.Client, error) {
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("error creating MinIO client: %v", err)
	}
	return minioClient, nil
}
