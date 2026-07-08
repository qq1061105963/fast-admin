package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SftpConfig 对应 SFTP 配置：password 和 privateKey 二选一。
type SftpConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	PrivateKey string `json:"privateKey"` // PEM 文本
	Passphrase string `json:"passphrase"`
	BasePath   string `json:"basePath"`
}

func (SftpConfig) isStorageConfig() {}

type SftpStorage struct{}

func NewSftpStorage() *SftpStorage { return &SftpStorage{} }

func (SftpStorage) Type() Type { return TypeSFTP }

func (SftpStorage) connect(cfg Config) (*sftp.Client, *ssh.Client, *SftpConfig, error) {
	c, ok := cfg.(*SftpConfig)
	if !ok {
		return nil, nil, nil, fmt.Errorf("storage: expected *SftpConfig, got %T", cfg)
	}
	port := c.Port
	if port == 0 {
		port = 22
	}

	var auth []ssh.AuthMethod
	if c.PrivateKey != "" {
		var signer ssh.Signer
		var err error
		if c.Passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(c.PrivateKey), []byte(c.Passphrase))
		} else {
			signer, err = ssh.ParsePrivateKey([]byte(c.PrivateKey))
		}
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse sftp private key: %w", err)
		}
		auth = append(auth, ssh.PublicKeys(signer))
	} else {
		auth = append(auth, ssh.Password(c.Password))
	}

	sshConfig := &ssh.ClientConfig{
		User: c.Username,
		Auth: auth,
		// 对应 Java 侧 StrictHostKeyChecking=no：不校验 known_hosts。
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second,
	}
	addr := fmt.Sprintf("%s:%d", c.Host, port)
	sshClient, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return nil, nil, nil, err
	}
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		sshClient.Close()
		return nil, nil, nil, err
	}
	return sftpClient, sshClient, c, nil
}

func (s SftpStorage) Upload(ctx context.Context, cfg Config, urlPrefix string, r io.Reader, meta Meta) (UploadResult, error) {
	sftpClient, sshClient, c, err := s.connect(cfg)
	if err != nil {
		return UploadResult{}, err
	}
	defer sftpClient.Close()
	defer sshClient.Close()

	key := objectKey(c.BasePath, meta.StorageKey)
	if dir := path.Dir(key); dir != "." {
		if err := sftpClient.MkdirAll(dir); err != nil {
			return UploadResult{}, err
		}
	}
	f, err := sftpClient.Create(key)
	if err != nil {
		return UploadResult{}, err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return UploadResult{}, err
	}
	return UploadResult{StorageKey: key, URL: JoinURL(urlPrefix, key)}, nil
}

// Delete 对不存在的文件视为删除成功（幂等），对应 Java 侧对 SSH_FX_NO_SUCH_FILE 的特殊处理。
func (s SftpStorage) Delete(ctx context.Context, cfg Config, storageKey string) error {
	sftpClient, sshClient, _, err := s.connect(cfg)
	if err != nil {
		return err
	}
	defer sftpClient.Close()
	defer sshClient.Close()

	if err := sftpClient.Remove(storageKey); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

type sftpReadCloser struct {
	file       *sftp.File
	sftpClient *sftp.Client
	sshClient  *ssh.Client
}

func (w *sftpReadCloser) Read(p []byte) (int, error) { return w.file.Read(p) }

func (w *sftpReadCloser) Close() error {
	_ = w.file.Close()
	_ = w.sftpClient.Close()
	return w.sshClient.Close()
}

func (s SftpStorage) Download(ctx context.Context, cfg Config, storageKey string) (io.ReadCloser, error) {
	sftpClient, sshClient, _, err := s.connect(cfg)
	if err != nil {
		return nil, err
	}
	f, err := sftpClient.Open(storageKey)
	if err != nil {
		sftpClient.Close()
		sshClient.Close()
		return nil, err
	}
	return &sftpReadCloser{file: f, sftpClient: sftpClient, sshClient: sshClient}, nil
}
