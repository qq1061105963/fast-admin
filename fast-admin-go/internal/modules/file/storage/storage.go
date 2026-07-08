// Package storage 定义多云文件存储的 SPI（策略接口），对应 Java 侧的
// FileStorage 接口 + FileStorageFactory。每种存储后端（本地/阿里云OSS/S3/SFTP/FTP）
// 实现同一个接口，由 Factory 按当前激活的配置选择具体实现。
package storage

import (
	"context"
	"io"
	"strings"
)

type Type string

const (
	TypeLocal Type = "LOCAL"
	TypeOSS   Type = "OSS"
	TypeS3    Type = "S3"
	TypeSFTP  Type = "SFTP"
	TypeFTP   Type = "FTP"
)

// Config 是各存储实现自己的配置结构的标记接口。
type Config interface {
	isStorageConfig()
}

// Meta 是上传时的元数据。
type Meta struct {
	OriginalName string
	StorageKey   string // 目标 key（含相对路径），如 2026/05/15/xxx.png
	Size         int64
	ContentType  string
}

// UploadResult 是上传结果：最终 key（实现可能调整，比如加 basePath 前缀）+ 完整访问地址。
type UploadResult struct {
	StorageKey string
	URL        string
}

// Storage 是所有存储后端的统一接口。
type Storage interface {
	Type() Type
	Upload(ctx context.Context, cfg Config, urlPrefix string, r io.Reader, meta Meta) (UploadResult, error)
	Delete(ctx context.Context, cfg Config, storageKey string) error
	Download(ctx context.Context, cfg Config, storageKey string) (io.ReadCloser, error)
}

// JoinURL 拼接访问地址：urlPrefix 去掉结尾 /，storageKey 去掉开头 /，用 / 连接。
func JoinURL(urlPrefix, storageKey string) string {
	return strings.TrimRight(urlPrefix, "/") + "/" + strings.TrimLeft(storageKey, "/")
}
