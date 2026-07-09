package file

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"time"

	"github.com/segmentio/ksuid"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/file/storage"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/fileconfig"
)

// ReferenceChecker 是文件删除前的引用检查 SPI，对应 Java 侧 FileReferenceChecker：
// 有引用则返回禁止删除的原因，无引用返回空字符串。当前没有内置实现（原本给
// AI 知识库这类不在本次迁移范围的模块扩展），留空 checkers 列表即可正常工作。
type ReferenceChecker interface {
	CheckReference(ctx context.Context, fileID string) (reason string, err error)
}

type UploadInput struct {
	Reader      io.Reader
	Filename    string
	Size        int64
	ContentType string
	BizType     string
	BizID       string
}

type Service struct {
	repo       *Repository
	configRepo *fileconfig.Repository
	factory    *storage.Factory
	checkers   []ReferenceChecker
}

func NewService(repo *Repository, configRepo *fileconfig.Repository, factory *storage.Factory, checkers ...ReferenceChecker) *Service {
	return &Service{repo: repo, configRepo: configRepo, factory: factory, checkers: checkers}
}

// AddChecker 在 bootstrap 里由后续初始化的模块（如 AI 知识库）回填引用检查器，
// 让被引用的源文件无法在文件管理里直接删除。
func (s *Service) AddChecker(checker ReferenceChecker) {
	s.checkers = append(s.checkers, checker)
}

func (s *Service) Page(ctx context.Context, q Query) ([]File, int64, error) {
	list, total, err := s.repo.Page(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func extractExt(filename string) string {
	idx := strings.LastIndex(filename, ".")
	if idx < 0 || idx == len(filename)-1 {
		return ""
	}
	ext := strings.ToLower(filename[idx+1:])
	if len(ext) > 16 {
		return ""
	}
	return ext
}

func (s *Service) Upload(ctx context.Context, in UploadInput) (*File, error) {
	data, err := io.ReadAll(in.Reader)
	if err != nil {
		return nil, errs.ErrBadRequest.Wrap(err)
	}

	ext := extractExt(in.Filename)
	storageKey := time.Now().Format("2006/01/02") + "/" + ksuid.New().String()
	if ext != "" {
		storageKey += "." + ext
	}

	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])

	active, err := s.factory.Current(ctx)
	if err != nil {
		return nil, errs.New(40701, 500, "未配置激活的文件存储")
	}

	meta := storage.Meta{OriginalName: in.Filename, StorageKey: storageKey, Size: in.Size, ContentType: in.ContentType}
	result, err := active.Storage.Upload(ctx, active.Config, active.URLPrefix, bytes.NewReader(data), meta)
	if err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}

	f := &File{
		OriginalName: in.Filename, StorageKey: result.StorageKey, URL: result.URL, Size: in.Size,
		ContentType: in.ContentType, Ext: ext, Hash: hash,
		StorageType: string(active.Storage.Type()), ConfigID: active.ConfigID,
		BizType: in.BizType, BizID: in.BizID,
	}
	if err := s.repo.Create(ctx, f); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	return f, nil
}

func (s *Service) resolveHistorical(ctx context.Context, f *File) (storage.Storage, storage.Config, error) {
	cfg, err := s.configRepo.GetByID(ctx, f.ConfigID)
	if err != nil {
		return nil, nil, err
	}
	return s.factory.Resolve(storage.Type(f.StorageType), []byte(cfg.RawConfig))
}

func (s *Service) Download(ctx context.Context, id string) (io.ReadCloser, *File, error) {
	f, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, nil, errs.ErrNotFound.Wrap(err)
	}
	st, cfg, err := s.resolveHistorical(ctx, f)
	if err != nil {
		return nil, nil, errs.ErrInternal.Wrap(err)
	}
	rc, err := st.Download(ctx, cfg, f.StorageKey)
	if err != nil {
		return nil, nil, errs.ErrInternal.Wrap(err)
	}
	return rc, f, nil
}

// Delete 先做引用检查，再尽力物理删除存储后端文件（失败也继续，避免脏数据
// 卡住主流程），最后逻辑删除数据库记录。
func (s *Service) Delete(ctx context.Context, id string) error {
	f, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return errs.ErrNotFound.Wrap(err)
	}

	for _, checker := range s.checkers {
		reason, err := checker.CheckReference(ctx, id)
		if err != nil {
			return errs.ErrInternal.Wrap(err)
		}
		if reason != "" {
			return errs.New(40702, 400, reason)
		}
	}

	if st, cfg, err := s.resolveHistorical(ctx, f); err == nil {
		_ = st.Delete(ctx, cfg, f.StorageKey)
	}

	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}
