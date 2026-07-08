package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type LocalConfig struct {
	BasePath string `json:"basePath"`
}

func (LocalConfig) isStorageConfig() {}

type LocalStorage struct{}

func NewLocalStorage() *LocalStorage { return &LocalStorage{} }

func (LocalStorage) Type() Type { return TypeLocal }

func (LocalStorage) resolve(cfg Config, storageKey string) (string, error) {
	c, ok := cfg.(*LocalConfig)
	if !ok {
		return "", fmt.Errorf("storage: expected *LocalConfig, got %T", cfg)
	}
	base, err := filepath.Abs(c.BasePath)
	if err != nil {
		return "", err
	}
	target := filepath.Clean(filepath.Join(base, storageKey))
	// 防路径穿越：目标必须仍在 base 目录之下。
	if target != base && !strings.HasPrefix(target, base+string(filepath.Separator)) {
		return "", errors.New("storage: invalid storage key (path traversal detected)")
	}
	return target, nil
}

func (s LocalStorage) Upload(ctx context.Context, cfg Config, urlPrefix string, r io.Reader, meta Meta) (UploadResult, error) {
	target, err := s.resolve(cfg, meta.StorageKey)
	if err != nil {
		return UploadResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return UploadResult{}, err
	}
	f, err := os.Create(target)
	if err != nil {
		return UploadResult{}, err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{StorageKey: meta.StorageKey, URL: JoinURL(urlPrefix, meta.StorageKey)}, nil
}

func (s LocalStorage) Delete(ctx context.Context, cfg Config, storageKey string) error {
	target, err := s.resolve(cfg, storageKey)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s LocalStorage) Download(ctx context.Context, cfg Config, storageKey string) (io.ReadCloser, error) {
	target, err := s.resolve(cfg, storageKey)
	if err != nil {
		return nil, err
	}
	return os.Open(target)
}
