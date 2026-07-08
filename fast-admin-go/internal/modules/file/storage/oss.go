package storage

import (
	"context"
	"fmt"
	"io"
	"strings"

	aliyunoss "github.com/aliyun/aliyun-oss-go-sdk/oss"
)

// OssConfig 对应阿里云 OSS 配置。
type OssConfig struct {
	Endpoint  string `json:"endpoint"`
	Bucket    string `json:"bucket"`
	AccessKey string `json:"accessKey"`
	SecretKey string `json:"secretKey"`
	BasePath  string `json:"basePath"`
}

func (OssConfig) isStorageConfig() {}

type OssStorage struct{}

func NewOssStorage() *OssStorage { return &OssStorage{} }

func (OssStorage) Type() Type { return TypeOSS }

func (OssStorage) bucket(cfg Config) (*aliyunoss.Bucket, string, error) {
	c, ok := cfg.(*OssConfig)
	if !ok {
		return nil, "", fmt.Errorf("storage: expected *OssConfig, got %T", cfg)
	}
	client, err := aliyunoss.New(c.Endpoint, c.AccessKey, c.SecretKey)
	if err != nil {
		return nil, "", err
	}
	bucket, err := client.Bucket(c.Bucket)
	if err != nil {
		return nil, "", err
	}
	return bucket, c.BasePath, nil
}

func objectKey(basePath, storageKey string) string {
	base := strings.Trim(basePath, "/")
	key := strings.TrimLeft(storageKey, "/")
	if base == "" {
		return key
	}
	return base + "/" + key
}

func (s OssStorage) Upload(ctx context.Context, cfg Config, urlPrefix string, r io.Reader, meta Meta) (UploadResult, error) {
	bucket, basePath, err := s.bucket(cfg)
	if err != nil {
		return UploadResult{}, err
	}
	key := objectKey(basePath, meta.StorageKey)

	opts := []aliyunoss.Option{}
	if meta.ContentType != "" {
		opts = append(opts, aliyunoss.ContentType(meta.ContentType))
	}
	if err := bucket.PutObject(key, r, opts...); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{StorageKey: key, URL: JoinURL(urlPrefix, key)}, nil
}

func (s OssStorage) Delete(ctx context.Context, cfg Config, storageKey string) error {
	bucket, _, err := s.bucket(cfg)
	if err != nil {
		return err
	}
	return bucket.DeleteObject(storageKey)
}

func (s OssStorage) Download(ctx context.Context, cfg Config, storageKey string) (io.ReadCloser, error) {
	bucket, _, err := s.bucket(cfg)
	if err != nil {
		return nil, err
	}
	return bucket.GetObject(storageKey)
}
