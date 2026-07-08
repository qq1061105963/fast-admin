package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// S3Config 对应 AWS S3 及兼容服务（如 MinIO）。
type S3Config struct {
	Endpoint        string `json:"endpoint"` // 兼容服务用，AWS 官方留空
	Region          string `json:"region"`
	Bucket          string `json:"bucket"`
	AccessKey       string `json:"accessKey"`
	SecretKey       string `json:"secretKey"`
	BasePath        string `json:"basePath"`
	PathStyleAccess bool   `json:"pathStyleAccess"` // MinIO 等通常需要 true
}

func (S3Config) isStorageConfig() {}

type S3Storage struct{}

func NewS3Storage() *S3Storage { return &S3Storage{} }

func (S3Storage) Type() Type { return TypeS3 }

func (S3Storage) client(cfg Config) (*s3.Client, *S3Config, error) {
	c, ok := cfg.(*S3Config)
	if !ok {
		return nil, nil, fmt.Errorf("storage: expected *S3Config, got %T", cfg)
	}
	client := s3.New(s3.Options{
		Region:       c.Region,
		Credentials:  credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, ""),
		UsePathStyle: c.PathStyleAccess,
		BaseEndpoint: nonEmptyPtr(c.Endpoint),
	})
	return client, c, nil
}

func nonEmptyPtr(s string) *string {
	if s == "" {
		return nil
	}
	return aws.String(s)
}

func (s S3Storage) Upload(ctx context.Context, cfg Config, urlPrefix string, r io.Reader, meta Meta) (UploadResult, error) {
	client, c, err := s.client(cfg)
	if err != nil {
		return UploadResult{}, err
	}
	key := objectKey(c.BasePath, meta.StorageKey)

	input := &s3.PutObjectInput{
		Bucket: aws.String(c.Bucket),
		Key:    aws.String(key),
		Body:   r,
	}
	if meta.ContentType != "" {
		input.ContentType = aws.String(meta.ContentType)
	}
	if _, err := client.PutObject(ctx, input); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{StorageKey: key, URL: JoinURL(urlPrefix, key)}, nil
}

func (s S3Storage) Delete(ctx context.Context, cfg Config, storageKey string) error {
	client, c, err := s.client(cfg)
	if err != nil {
		return err
	}
	_, err = client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(c.Bucket), Key: aws.String(storageKey),
	})
	return err
}

func (s S3Storage) Download(ctx context.Context, cfg Config, storageKey string) (io.ReadCloser, error) {
	client, c, err := s.client(cfg)
	if err != nil {
		return nil, err
	}
	resp, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.Bucket), Key: aws.String(storageKey),
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}
