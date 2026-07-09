package rag

import (
	"context"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/SirYuxuan/fast-admin-go/internal/framework/errs"
	"github.com/SirYuxuan/fast-admin-go/internal/framework/logger"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/ai/settings"
	"github.com/SirYuxuan/fast-admin-go/internal/modules/file"
)

const (
	defaultChunkSize    = 800
	defaultChunkOverlap = 100
	defaultChunkDelim   = `\n\n`
	maxChunkSize        = 4000
	maxTopK             = 20
	chatTopK            = 4
	fileBizType         = "AI_RAG_DOCUMENT"
)

// FileService 抽象文件模块能力，便于 RAG 上传/下载/删除源文件。
type FileService interface {
	Upload(ctx context.Context, in file.UploadInput) (*file.File, error)
	Download(ctx context.Context, id string) (io.ReadCloser, *file.File, error)
	Delete(ctx context.Context, id string) error
}

type Service struct {
	repo      *Repository
	embedding *Embedding
	qdrant    *Qdrant
	set       *settings.Settings
	fileSvc   FileService
}

func NewService(repo *Repository, embedding *Embedding, qdrant *Qdrant, set *settings.Settings, fileSvc FileService) *Service {
	return &Service{repo: repo, embedding: embedding, qdrant: qdrant, set: set, fileSvc: fileSvc}
}

// ---- 知识库 CRUD ----

type KBSaveDTO struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Enabled        *bool  `json:"enabled"`
	ChunkSize      *int   `json:"chunkSize"`
	ChunkOverlap   *int   `json:"chunkOverlap"`
	ChunkDelimiter string `json:"chunkDelimiter"`
	Remark         string `json:"remark"`
}

func (s *Service) PageKB(ctx context.Context, q KBQuery) ([]AiKnowledgeBase, int64, error) {
	list, total, err := s.repo.PageKB(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) GetKBOrErr(ctx context.Context, id string) (*AiKnowledgeBase, error) {
	kb, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errs.New(40404, 404, "知识库不存在")
	}
	return kb, nil
}

func (s *Service) AddKB(ctx context.Context, dto *KBSaveDTO) error {
	if err := validateKB(dto); err != nil {
		return err
	}
	exists, err := s.repo.KBNameExists(ctx, "", dto.Name)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if exists {
		return errs.New(40110, 400, "知识库名称已存在")
	}
	kb := &AiKnowledgeBase{}
	copyToKB(dto, kb)
	kb.Enabled = dto.Enabled == nil || *dto.Enabled
	kb.DocumentCount = 0
	kb.ChunkCount = 0
	if err := s.repo.Create(ctx, kb); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) UpdateKB(ctx context.Context, dto *KBSaveDTO) error {
	kb, err := s.GetKBOrErr(ctx, dto.ID)
	if err != nil {
		return err
	}
	if err := validateKB(dto); err != nil {
		return err
	}
	exists, err := s.repo.KBNameExists(ctx, kb.ID, dto.Name)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	if exists {
		return errs.New(40110, 400, "知识库名称已存在")
	}
	copyToKB(dto, kb)
	if err := s.repo.Update(ctx, kb); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) ChangeEnabled(ctx context.Context, id string, enabled bool) error {
	kb, err := s.GetKBOrErr(ctx, id)
	if err != nil {
		return err
	}
	kb.Enabled = enabled
	if err := s.repo.Update(ctx, kb); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

func (s *Service) DelKB(ctx context.Context, id string) error {
	if _, err := s.GetKBOrErr(ctx, id); err != nil {
		return err
	}
	docs, err := s.repo.ListDocByKB(ctx, id)
	if err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	for i := range docs {
		_ = s.DeleteDocument(ctx, docs[i].ID, true)
	}
	if err := s.repo.DeleteByID(ctx, id); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	return nil
}

// ---- 文档 ----

func (s *Service) DocumentPage(ctx context.Context, q DocQuery) ([]AiKnowledgeDocument, int64, error) {
	list, total, err := s.repo.PageDoc(ctx, q)
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) GetDocumentDetail(ctx context.Context, id string) (*AiKnowledgeDocument, error) {
	doc, err := s.repo.GetDoc(ctx, id)
	if err != nil {
		return nil, errs.New(40404, 404, "知识库文档不存在")
	}
	return doc, nil
}

func (s *Service) ChunkPage(ctx context.Context, docID string, page, size int) ([]AiKnowledgeChunk, int64, error) {
	doc, err := s.repo.GetDoc(ctx, docID)
	if err != nil {
		return nil, 0, errs.New(40404, 404, "知识库文档不存在")
	}
	list, total, err := s.repo.PageChunk(ctx, ChunkQuery{DocumentID: doc.ID, Page: page, Size: size})
	if err != nil {
		return nil, 0, errs.ErrInternal.Wrap(err)
	}
	return list, total, nil
}

func (s *Service) UploadDocument(ctx context.Context, kbID string, in file.UploadInput) (*AiKnowledgeDocument, error) {
	if err := s.ensureRagEnabled(ctx); err != nil {
		return nil, err
	}
	kb, err := s.GetKBOrErr(ctx, kbID)
	if err != nil {
		return nil, err
	}
	in.BizType = fileBizType
	in.BizID = kb.ID
	saved, err := s.fileSvc.Upload(ctx, in)
	if err != nil {
		return nil, err
	}
	size := saved.Size
	doc := &AiKnowledgeDocument{
		KnowledgeBaseID: kb.ID,
		FileID:          saved.ID,
		FileName:        saved.OriginalName,
		ContentType:     saved.ContentType,
		FileSize:        &size,
		Status:          StatusPending,
		ChunkCount:      0,
	}
	if err := s.repo.CreateDoc(ctx, doc); err != nil {
		return nil, errs.ErrInternal.Wrap(err)
	}
	_ = s.refreshStats(ctx, kb.ID)
	s.asyncIndex(doc.ID)
	return doc, nil
}

func (s *Service) ReindexDocument(ctx context.Context, docID string) error {
	if err := s.ensureRagEnabled(ctx); err != nil {
		return err
	}
	doc, err := s.repo.GetDoc(ctx, docID)
	if err != nil {
		return errs.New(40404, 404, "知识库文档不存在")
	}
	if err := s.repo.UpdateDocStatus(ctx, doc.ID, StatusPending, 0, "", nil); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	_ = s.refreshStats(ctx, doc.KnowledgeBaseID)
	s.asyncIndex(doc.ID)
	return nil
}

func (s *Service) DeleteDocument(ctx context.Context, docID string, deleteSourceFile bool) error {
	doc, err := s.repo.GetDoc(ctx, docID)
	if err != nil {
		return errs.New(40404, 404, "知识库文档不存在")
	}
	if err := s.deleteChunks(ctx, doc.ID); err != nil {
		logger.L().Sugar().Warnf("删除切片失败 docId=%s: %v", doc.ID, err)
	}
	if err := s.repo.DeleteDoc(ctx, docID); err != nil {
		return errs.ErrInternal.Wrap(err)
	}
	_ = s.refreshStats(ctx, doc.KnowledgeBaseID)
	if deleteSourceFile && strings.TrimSpace(doc.FileID) != "" {
		_ = s.fileSvc.Delete(ctx, doc.FileID)
	}
	return nil
}

// ---- 向量库状态 ----

func (s *Service) VectorStoreStatus(ctx context.Context) *VectorStoreStatus {
	return s.qdrant.Status(ctx)
}

func (s *Service) Collections(ctx context.Context) ([]string, error) {
	return s.qdrant.Collections(ctx)
}

// ---- 召回测试 ----

type RecallResult struct {
	Query     string       `json:"query"`
	TopK      int          `json:"topK"`
	LatencyMs int64        `json:"latencyMs"`
	Items     []RecallItem `json:"items"`
}

type RecallItem struct {
	ChunkID    string  `json:"chunkId"`
	DocumentID string  `json:"documentId"`
	FileName   string  `json:"fileName"`
	ChunkIndex int     `json:"chunkIndex"`
	Score      float64 `json:"score"`
	Content    string  `json:"content"`
}

func (s *Service) Recall(ctx context.Context, kbID, query string, topK *int) (*RecallResult, error) {
	if err := s.ensureRagEnabled(ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(kbID) == "" {
		return nil, errs.New(40111, 400, "知识库不能为空")
	}
	if strings.TrimSpace(query) == "" {
		return nil, errs.New(40112, 400, "召回问题不能为空")
	}
	kb, err := s.GetKBOrErr(ctx, kbID)
	if err != nil {
		return nil, err
	}
	if !kb.Enabled {
		return nil, errs.New(40113, 400, "知识库已禁用")
	}
	k := 5
	if topK != nil {
		k = *topK
	}
	k = clampRag(k, 1, maxTopK)
	start := time.Now()
	vector, err := s.embedding.Embed(ctx, query)
	if err != nil {
		return nil, err
	}
	hits, err := s.qdrant.Search(ctx, vector, kb.ID, k)
	if err != nil {
		return nil, err
	}
	pointIDs := make([]string, 0, len(hits))
	for _, h := range hits {
		pointIDs = append(pointIDs, h.PointID)
	}
	chunks, _ := s.repo.ListChunkByPointIDs(ctx, pointIDs)
	byPoint := map[string]AiKnowledgeChunk{}
	for _, c := range chunks {
		byPoint[c.PointID] = c
	}
	docs := s.loadDocuments(ctx, chunks)

	items := make([]RecallItem, 0, len(hits))
	for _, h := range hits {
		chunk, ok := byPoint[h.PointID]
		if !ok {
			continue
		}
		fileName := ""
		if doc, ok := docs[chunk.DocumentID]; ok {
			fileName = doc.FileName
		}
		items = append(items, RecallItem{
			ChunkID: chunk.ID, DocumentID: chunk.DocumentID, FileName: fileName,
			ChunkIndex: chunk.ChunkIndex, Score: h.Score, Content: chunk.Content,
		})
	}
	return &RecallResult{Query: query, TopK: k, LatencyMs: time.Since(start).Milliseconds(), Items: items}, nil
}

// ChatContext 是注入聊天提示词的知识库上下文。
type ChatContext struct {
	Text    string
	Sources []string
}

// RetrieveChatContext 为聊天检索知识库上下文；失败/无命中返回 nil，不影响对话。
func (s *Service) RetrieveChatContext(ctx context.Context, ragMode string, kbIDs []string, query string) *ChatContext {
	if strings.EqualFold(ragMode, "off") || strings.TrimSpace(query) == "" || !s.set.RagEnabled(ctx) {
		return nil
	}
	var kbs []AiKnowledgeBase
	var err error
	if strings.EqualFold(ragMode, "manual") && len(kbIDs) > 0 {
		all, e := s.repo.ListKBByIDs(ctx, kbIDs)
		err = e
		for _, kb := range all {
			if kb.Enabled {
				kbs = append(kbs, kb)
			}
		}
	} else {
		kbs, err = s.repo.ListEnabledKB(ctx)
	}
	if err != nil || len(kbs) == 0 {
		return nil
	}
	vector, err := s.embedding.Embed(ctx, query)
	if err != nil {
		logger.L().Sugar().Warnf("知识库检索失败，本轮对话跳过 RAG：%v", err)
		return nil
	}
	var hits []SearchHit
	for _, kb := range kbs {
		h, err := s.qdrant.Search(ctx, vector, kb.ID, chatTopK)
		if err != nil {
			logger.L().Sugar().Warnf("知识库检索失败，本轮对话跳过 RAG：%v", err)
			return nil
		}
		hits = append(hits, h...)
	}
	if len(hits) == 0 {
		return nil
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if len(hits) > chatTopK {
		hits = hits[:chatTopK]
	}
	pointIDs := make([]string, 0, len(hits))
	for _, h := range hits {
		pointIDs = append(pointIDs, h.PointID)
	}
	chunks, _ := s.repo.ListChunkByPointIDs(ctx, pointIDs)
	if len(chunks) == 0 {
		return nil
	}
	byPoint := map[string]AiKnowledgeChunk{}
	for _, c := range chunks {
		byPoint[c.PointID] = c
	}
	docs := s.loadDocuments(ctx, chunks)

	var sb strings.Builder
	sb.WriteString("以下是从知识库检索到的参考资料，回答时请优先依据它们；若与问题无关请忽略：\n")
	var sources []string
	idx := 1
	for _, pid := range pointIDs {
		chunk, ok := byPoint[pid]
		if !ok {
			continue
		}
		fileName := "未知文档"
		if doc, ok := docs[chunk.DocumentID]; ok {
			fileName = doc.FileName
		}
		sb.WriteString("\n[")
		sb.WriteString(itoa(idx))
		sb.WriteString("] 来源：")
		sb.WriteString(fileName)
		sb.WriteByte('\n')
		sb.WriteString(chunk.Content)
		sb.WriteByte('\n')
		idx++
		if !containsStr(sources, fileName) {
			sources = append(sources, fileName)
		}
	}
	return &ChatContext{Text: sb.String(), Sources: sources}
}

// ---- 索引 ----

func (s *Service) asyncIndex(docID string) {
	go func() {
		s.indexDocument(context.Background(), docID)
	}()
}

// indexDocument 抽取文本 → 切片 → embedding → 写 Qdrant + 切片表，失败落 failed 状态。
func (s *Service) indexDocument(ctx context.Context, docID string) {
	doc, err := s.repo.GetDoc(ctx, docID)
	if err != nil {
		return
	}
	kb, err := s.repo.GetByID(ctx, doc.KnowledgeBaseID)
	if err != nil {
		return
	}
	_ = s.repo.UpdateDocStatus(ctx, doc.ID, StatusIndexing, 0, "", nil)
	_ = s.deleteChunks(ctx, doc.ID)

	fail := func(msg string) {
		_ = s.repo.UpdateDocStatus(ctx, doc.ID, StatusFailed, 0, msg, nil)
		_ = s.refreshStats(ctx, kb.ID)
	}

	text, err := s.extractText(ctx, doc)
	if err != nil {
		fail(err.Error())
		return
	}
	chunks := splitText(text, kbChunkSize(kb), kbChunkOverlap(kb), kbChunkDelimiter(kb))
	if len(chunks) == 0 {
		fail("文档没有可索引文本")
		return
	}
	firstVector, err := s.embedding.Embed(ctx, chunks[0])
	if err != nil {
		fail(err.Error())
		return
	}
	if err := s.qdrant.EnsureCollection(ctx, len(firstVector)); err != nil {
		fail(err.Error())
		return
	}
	if err := s.saveChunk(ctx, kb, doc, 0, chunks[0], firstVector); err != nil {
		fail(err.Error())
		return
	}
	for i := 1; i < len(chunks); i++ {
		vec, err := s.embedding.Embed(ctx, chunks[i])
		if err != nil {
			fail(err.Error())
			return
		}
		if err := s.saveChunk(ctx, kb, doc, i, chunks[i], vec); err != nil {
			fail(err.Error())
			return
		}
	}
	now := time.Now()
	_ = s.repo.UpdateDocStatus(ctx, doc.ID, StatusIndexed, len(chunks), "", &now)
	_ = s.refreshStats(ctx, kb.ID)
}

func (s *Service) saveChunk(ctx context.Context, kb *AiKnowledgeBase, doc *AiKnowledgeDocument, index int, content string, vector []float64) error {
	pointID := uuid.NewString()
	tokens := estimateTokens(content)
	chunk := &AiKnowledgeChunk{
		KnowledgeBaseID: kb.ID,
		DocumentID:      doc.ID,
		PointID:         pointID,
		ChunkIndex:      index,
		TokenCount:      &tokens,
		Content:         content,
	}
	if err := s.repo.CreateChunk(ctx, chunk); err != nil {
		return err
	}
	payload := map[string]any{
		"knowledgeBaseId": kb.ID,
		"documentId":      doc.ID,
		"chunkId":         chunk.ID,
		"chunkIndex":      index,
		"fileName":        doc.FileName,
	}
	return s.qdrant.Upsert(ctx, pointID, vector, payload)
}

func (s *Service) extractText(ctx context.Context, doc *AiKnowledgeDocument) (string, error) {
	rc, f, err := s.fileSvc.Download(ctx, doc.FileID)
	if err != nil {
		return "", errs.New(40114, 400, "源文件不存在")
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return "", errs.New(40115, 400, "读取文档失败："+err.Error())
	}
	return extractText(f.Ext, data)
}

func (s *Service) deleteChunks(ctx context.Context, docID string) error {
	chunks, err := s.repo.ListChunkByDoc(ctx, docID)
	if err != nil {
		return err
	}
	pointIDs := make([]string, 0, len(chunks))
	for _, c := range chunks {
		pointIDs = append(pointIDs, c.PointID)
	}
	_ = s.qdrant.DeletePoints(ctx, pointIDs)
	return s.repo.DeleteChunkByDoc(ctx, docID)
}

func (s *Service) refreshStats(ctx context.Context, kbID string) error {
	docCount, err := s.repo.CountDoc(ctx, kbID)
	if err != nil {
		return err
	}
	chunkCount, err := s.repo.CountChunk(ctx, kbID)
	if err != nil {
		return err
	}
	lastIndexedAt, _ := s.repo.MaxIndexedAt(ctx, kbID)
	return s.repo.UpdateStats(ctx, kbID, int(docCount), int(chunkCount), lastIndexedAt)
}

func (s *Service) loadDocuments(ctx context.Context, chunks []AiKnowledgeChunk) map[string]AiKnowledgeDocument {
	var docIDs []string
	seen := map[string]bool{}
	for _, c := range chunks {
		if !seen[c.DocumentID] {
			seen[c.DocumentID] = true
			docIDs = append(docIDs, c.DocumentID)
		}
	}
	out := map[string]AiKnowledgeDocument{}
	if len(docIDs) == 0 {
		return out
	}
	docs, _ := s.repo.ListDocByIDs(ctx, docIDs)
	for _, d := range docs {
		out[d.ID] = d
	}
	return out
}

func (s *Service) ensureRagEnabled(ctx context.Context) error {
	if !s.set.RagEnabled(ctx) {
		return errs.New(40116, 400, "AI 知识库未启用")
	}
	return nil
}

// CheckReference 实现 file.ReferenceChecker：被知识库文档引用的源文件禁止直接删除。
func (s *Service) CheckReference(ctx context.Context, fileID string) (string, error) {
	name, found, err := s.repo.FindDocFileNameByFileID(ctx, fileID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	return "该文件已被 AI 知识库文档「" + name + "」引用，请先在知识库中删除对应文档后再删除源文件", nil
}
