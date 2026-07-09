package rag

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/crud"
)

// Repository 汇总知识库 / 文档 / 切片三张表的读写。
type Repository struct {
	*crud.BaseRepo[AiKnowledgeBase]
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{BaseRepo: crud.NewBaseRepo[AiKnowledgeBase](db), db: db}
}

// ---- 知识库 ----

type KBQuery struct {
	Name    string
	Enabled *bool
	Page    int
	Size    int
}

func (r *Repository) PageKB(ctx context.Context, q KBQuery) ([]AiKnowledgeBase, int64, error) {
	scope := func(db *gorm.DB) *gorm.DB {
		if q.Name != "" {
			db = db.Where("name LIKE ?", "%"+q.Name+"%")
		}
		if q.Enabled != nil {
			db = db.Where("enabled = ?", *q.Enabled)
		}
		return db.Order("created_at DESC")
	}
	return r.BaseRepo.Page(ctx, q.Page, q.Size, scope)
}

func (r *Repository) KBNameExists(ctx context.Context, excludeID, name string) (bool, error) {
	q := r.db.WithContext(ctx).Model(&AiKnowledgeBase{}).Where("name = ?", name)
	if excludeID != "" {
		q = q.Where("id <> ?", excludeID)
	}
	var count int64
	err := q.Count(&count).Error
	return count > 0, err
}

func (r *Repository) ListEnabledKB(ctx context.Context) ([]AiKnowledgeBase, error) {
	var list []AiKnowledgeBase
	err := r.db.WithContext(ctx).Where("enabled = ?", true).Find(&list).Error
	return list, err
}

func (r *Repository) ListKBByIDs(ctx context.Context, ids []string) ([]AiKnowledgeBase, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []AiKnowledgeBase
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&list).Error
	return list, err
}

// UpdateStats 回填文档数 / 切片数 / 最后索引时间（lastIndexedAt 可为 nil 清空）。
func (r *Repository) UpdateStats(ctx context.Context, kbID string, docCount, chunkCount int, lastIndexedAt *time.Time) error {
	return r.db.WithContext(ctx).Model(&AiKnowledgeBase{}).Where("id = ?", kbID).Updates(map[string]any{
		"document_count":  docCount,
		"chunk_count":     chunkCount,
		"last_indexed_at": lastIndexedAt,
	}).Error
}

// ---- 文档 ----

type DocQuery struct {
	KnowledgeBaseID string
	Status          string
	Page            int
	Size            int
}

func (r *Repository) PageDoc(ctx context.Context, q DocQuery) ([]AiKnowledgeDocument, int64, error) {
	tx := r.db.WithContext(ctx).Model(&AiKnowledgeDocument{})
	if q.KnowledgeBaseID != "" {
		tx = tx.Where("knowledge_base_id = ?", q.KnowledgeBaseID)
	}
	if q.Status != "" {
		tx = tx.Where("status = ?", q.Status)
	}
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := q.Page, q.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	var list []AiKnowledgeDocument
	err := tx.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

func (r *Repository) GetDoc(ctx context.Context, id string) (*AiKnowledgeDocument, error) {
	var d AiKnowledgeDocument
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&d).Error; err != nil {
		return nil, err
	}
	return &d, nil
}

func (r *Repository) CreateDoc(ctx context.Context, d *AiKnowledgeDocument) error {
	return r.db.WithContext(ctx).Create(d).Error
}

func (r *Repository) ListDocByKB(ctx context.Context, kbID string) ([]AiKnowledgeDocument, error) {
	var list []AiKnowledgeDocument
	err := r.db.WithContext(ctx).Where("knowledge_base_id = ?", kbID).Find(&list).Error
	return list, err
}

func (r *Repository) ListDocByIDs(ctx context.Context, ids []string) ([]AiKnowledgeDocument, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []AiKnowledgeDocument
	err := r.db.WithContext(ctx).Where("id IN ?", ids).Find(&list).Error
	return list, err
}

func (r *Repository) DeleteDoc(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Where("id = ?", id).Delete(&AiKnowledgeDocument{}).Error
}

func (r *Repository) CountDoc(ctx context.Context, kbID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&AiKnowledgeDocument{}).Where("knowledge_base_id = ?", kbID).Count(&count).Error
	return count, err
}

// UpdateDocStatus 更新文档状态相关字段，errorMsg/indexedAt/chunkCount 都会写入（含清空）。
func (r *Repository) UpdateDocStatus(ctx context.Context, id, status string, chunkCount int, errorMsg string, indexedAt *time.Time) error {
	return r.db.WithContext(ctx).Model(&AiKnowledgeDocument{}).Where("id = ?", id).Updates(map[string]any{
		"status":      status,
		"chunk_count": chunkCount,
		"error_msg":   nullIfEmpty(errorMsg),
		"indexed_at":  indexedAt,
	}).Error
}

// MaxIndexedAt 返回该知识库下已索引文档的最新 indexed_at，无则 nil。
func (r *Repository) MaxIndexedAt(ctx context.Context, kbID string) (*time.Time, error) {
	var t *time.Time
	err := r.db.WithContext(ctx).Model(&AiKnowledgeDocument{}).
		Where("knowledge_base_id = ? AND status = ? AND indexed_at IS NOT NULL", kbID, StatusIndexed).
		Select("MAX(indexed_at)").Scan(&t).Error
	return t, err
}

// ---- 切片 ----

func (r *Repository) CreateChunk(ctx context.Context, c *AiKnowledgeChunk) error {
	return r.db.WithContext(ctx).Create(c).Error
}

func (r *Repository) ListChunkByDoc(ctx context.Context, docID string) ([]AiKnowledgeChunk, error) {
	var list []AiKnowledgeChunk
	err := r.db.WithContext(ctx).Where("document_id = ?", docID).Find(&list).Error
	return list, err
}

func (r *Repository) ListChunkByPointIDs(ctx context.Context, pointIDs []string) ([]AiKnowledgeChunk, error) {
	if len(pointIDs) == 0 {
		return nil, nil
	}
	var list []AiKnowledgeChunk
	err := r.db.WithContext(ctx).Where("point_id IN ?", pointIDs).Find(&list).Error
	return list, err
}

func (r *Repository) DeleteChunkByDoc(ctx context.Context, docID string) error {
	return r.db.WithContext(ctx).Where("document_id = ?", docID).Delete(&AiKnowledgeChunk{}).Error
}

func (r *Repository) CountChunk(ctx context.Context, kbID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&AiKnowledgeChunk{}).Where("knowledge_base_id = ?", kbID).Count(&count).Error
	return count, err
}

type ChunkQuery struct {
	DocumentID string
	Page       int
	Size       int
}

func (r *Repository) PageChunk(ctx context.Context, q ChunkQuery) ([]AiKnowledgeChunk, int64, error) {
	tx := r.db.WithContext(ctx).Model(&AiKnowledgeChunk{}).Where("document_id = ?", q.DocumentID)
	var total int64
	if err := tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	page, size := q.Page, q.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 10
	}
	var list []AiKnowledgeChunk
	err := tx.Order("chunk_index ASC").Offset((page - 1) * size).Limit(size).Find(&list).Error
	return list, total, err
}

// CountDocByFileID 用于文件引用检查：返回引用该源文件的文档名（存在时）。
func (r *Repository) FindDocFileNameByFileID(ctx context.Context, fileID string) (string, bool, error) {
	var d AiKnowledgeDocument
	err := r.db.WithContext(ctx).Select("file_name").Where("file_id = ?", fileID).Limit(1).First(&d).Error
	if err == gorm.ErrRecordNotFound {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return d.FileName, true, nil
}

func nullIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
