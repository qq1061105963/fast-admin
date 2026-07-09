package rag

import (
	"time"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/model"
)

// AiKnowledgeBase 对应 ai_knowledge_base。
type AiKnowledgeBase struct {
	model.BaseModel
	Name           string     `gorm:"column:name" json:"name"`
	Description    string     `gorm:"column:description" json:"description"`
	Enabled        bool       `gorm:"column:enabled" json:"enabled"`
	ChunkSize      int        `gorm:"column:chunk_size" json:"chunkSize"`
	ChunkOverlap   int        `gorm:"column:chunk_overlap" json:"chunkOverlap"`
	ChunkDelimiter string     `gorm:"column:chunk_delimiter" json:"chunkDelimiter"`
	DocumentCount  int        `gorm:"column:document_count" json:"documentCount"`
	ChunkCount     int        `gorm:"column:chunk_count" json:"chunkCount"`
	LastIndexedAt  *time.Time `gorm:"column:last_indexed_at" json:"lastIndexedAt"`
	Remark         string     `gorm:"column:remark" json:"remark"`
}

func (AiKnowledgeBase) TableName() string { return "ai_knowledge_base" }

// 文档索引状态。
const (
	StatusPending  = "pending"
	StatusIndexing = "indexing"
	StatusIndexed  = "indexed"
	StatusFailed   = "failed"
)

// AiKnowledgeDocument 对应 ai_knowledge_document。
type AiKnowledgeDocument struct {
	model.BaseModel
	KnowledgeBaseID string     `gorm:"column:knowledge_base_id" json:"knowledgeBaseId"`
	FileID          string     `gorm:"column:file_id" json:"fileId"`
	FileName        string     `gorm:"column:file_name" json:"fileName"`
	ContentType     string     `gorm:"column:content_type" json:"contentType"`
	FileSize        *int64     `gorm:"column:file_size" json:"fileSize"`
	Status          string     `gorm:"column:status" json:"status"`
	ChunkCount      int        `gorm:"column:chunk_count" json:"chunkCount"`
	ErrorMsg        string     `gorm:"column:error_msg" json:"errorMsg"`
	IndexedAt       *time.Time `gorm:"column:indexed_at" json:"indexedAt"`
}

func (AiKnowledgeDocument) TableName() string { return "ai_knowledge_document" }

// AiKnowledgeChunk 对应 ai_knowledge_chunk。
type AiKnowledgeChunk struct {
	model.BaseModel
	KnowledgeBaseID string `gorm:"column:knowledge_base_id" json:"knowledgeBaseId"`
	DocumentID      string `gorm:"column:document_id" json:"documentId"`
	PointID         string `gorm:"column:point_id" json:"pointId"`
	ChunkIndex      int    `gorm:"column:chunk_index" json:"chunkIndex"`
	TokenCount      *int   `gorm:"column:token_count" json:"tokenCount"`
	Content         string `gorm:"column:content" json:"content"`
}

func (AiKnowledgeChunk) TableName() string { return "ai_knowledge_chunk" }
