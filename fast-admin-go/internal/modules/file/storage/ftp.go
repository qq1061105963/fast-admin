package storage

import (
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/jlaffaye/ftp"
)

type FtpConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	BasePath string `json:"basePath"`
	Passive  bool   `json:"passive"`
}

func (FtpConfig) isStorageConfig() {}

type FtpStorage struct{}

func NewFtpStorage() *FtpStorage { return &FtpStorage{} }

func (FtpStorage) Type() Type { return TypeFTP }

func (FtpStorage) connect(cfg Config) (*ftp.ServerConn, *FtpConfig, error) {
	c, ok := cfg.(*FtpConfig)
	if !ok {
		return nil, nil, fmt.Errorf("storage: expected *FtpConfig, got %T", cfg)
	}
	port := c.Port
	if port == 0 {
		port = 21
	}
	addr := fmt.Sprintf("%s:%d", c.Host, port)
	conn, err := ftp.Dial(addr, ftp.DialWithTimeout(10*time.Second))
	if err != nil {
		return nil, nil, err
	}
	if err := conn.Login(c.Username, c.Password); err != nil {
		conn.Quit()
		return nil, nil, err
	}
	return conn, c, nil
}

// ensureDir 递归创建远端多级目录，FTP 协议没有 mkdir -p，逐级 MakeDir，
// 已存在的目录会报错但被忽略（best-effort，语义对应 Java 侧的 ensureDir）。
func ensureFtpDir(conn *ftp.ServerConn, dir string) {
	dir = strings.Trim(dir, "/")
	if dir == "" || dir == "." {
		return
	}
	cur := ""
	for _, p := range strings.Split(dir, "/") {
		cur += "/" + p
		_ = conn.MakeDir(cur)
	}
}

func (s FtpStorage) Upload(ctx context.Context, cfg Config, urlPrefix string, r io.Reader, meta Meta) (UploadResult, error) {
	conn, c, err := s.connect(cfg)
	if err != nil {
		return UploadResult{}, err
	}
	defer conn.Quit()

	key := objectKey(c.BasePath, meta.StorageKey)
	ensureFtpDir(conn, path.Dir(key))
	if err := conn.Stor(key, r); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{StorageKey: key, URL: JoinURL(urlPrefix, key)}, nil
}

func (s FtpStorage) Delete(ctx context.Context, cfg Config, storageKey string) error {
	conn, _, err := s.connect(cfg)
	if err != nil {
		return err
	}
	defer conn.Quit()
	if err := conn.Delete(storageKey); err != nil {
		return err
	}
	return nil
}

// ftpReadCloser 把 *ftp.Response 和背后的控制连接绑在一起，Close 时一并释放。
type ftpReadCloser struct {
	resp *ftp.Response
	conn *ftp.ServerConn
}

func (w *ftpReadCloser) Read(p []byte) (int, error) { return w.resp.Read(p) }

func (w *ftpReadCloser) Close() error {
	_ = w.resp.Close()
	return w.conn.Quit()
}

func (s FtpStorage) Download(ctx context.Context, cfg Config, storageKey string) (io.ReadCloser, error) {
	conn, _, err := s.connect(cfg)
	if err != nil {
		return nil, err
	}
	resp, err := conn.Retr(storageKey)
	if err != nil {
		conn.Quit()
		return nil, err
	}
	return &ftpReadCloser{resp: resp, conn: conn}, nil
}
