package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/file_store"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"
)

// DocumentIndexer Document indexing service
type DocumentIndexer struct {
	Config      *config.IndexerConfig
	VectorStore vector_store.VectorStore
}

// BatchIndexReq Batch indexing request parameters
type BatchIndexReq struct {
	ModelID     string   // Embedding Model ID
	DocumentIds []string // Document ID list
	ChunkSize   int      // Document chunk size
	OverlapSize int      // Chunk overlap size
	Separator   string   // Custom separator
}

// IndexReq Unified indexing request parameters
type IndexReq struct {
	ModelID     string // Embedding Model ID
	DocumentId  string // Document ID
	ChunkSize   int    // Document chunk size
	OverlapSize int    // Chunk overlap size
	Separator   string // Custom separator
}

// indexContext Indexing context, used to pass data between pipeline steps
type indexContext struct {
	ctx            context.Context
	modelID        string
	documentId     string
	doc            gormModel.KnowledgeDocuments
	storageType    file_store.StorageType
	localFilePath  string
	chunks         []*schema.Document
	chunkSize      int
	overlapSize    int
	separator      string
	collectionName string
}

// IndexResult 索引结果
type IndexResult struct {
	DocumentID string
	Success    bool
	Error      error
}

// BatchDocumentIndex Batch document indexing processing
func (s *DocumentIndexer) BatchDocumentIndex(ctx context.Context, req *BatchIndexReq) error {
	// 使用 WaitGroup 管理 goroutines
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // 限制并发数为5
	results := make(chan IndexResult, len(req.DocumentIds))

	for _, documentId := range req.DocumentIds {
		wg.Add(1)
		documentId := documentId // 捕获循环变量
		common.SafeGo(ctx, fmt.Sprintf("IndexDoc-%s", documentId), func() {
			defer wg.Done()

			// 获取并发许可
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			indexReq := &IndexReq{
				ModelID:     req.ModelID,
				DocumentId:  documentId,
				ChunkSize:   req.ChunkSize,
				OverlapSize: req.OverlapSize,
				Separator:   req.Separator,
			}

			err := s.DocumentIndex(ctx, indexReq)

			// 发送结果
			results <- IndexResult{
				DocumentID: documentId,
				Success:    err == nil,
				Error:      err,
			}

			if err != nil {
				g.Log().Errorf(ctx, "Document indexing failed, documentId=%s, err=%v", documentId, err)
			}
		})
	}

	// 收集结果
	go func() {
		wg.Wait()
		close(results)
	}()

	// 统计结果
	go func() {
		successCount := 0
		failCount := 0
		for result := range results {
			if result.Success {
				successCount++
			} else {
				failCount++
			}
		}
		g.Log().Infof(ctx, "Batch document indexing completed: success=%d, failed=%d, total=%d",
			successCount, failCount, len(req.DocumentIds))
	}()

	return nil
}

// DocumentIndex Unified document indexing processing
func (s *DocumentIndexer) DocumentIndex(ctx context.Context, req *IndexReq) error {
	// Create indexing context
	idxCtx := &indexContext{
		ctx:         ctx,
		modelID:     req.ModelID,
		documentId:  req.DocumentId,
		chunkSize:   req.ChunkSize,
		overlapSize: req.OverlapSize,
		separator:   req.Separator,
	}

	// 将文档状态设置为"索引中"
	err := knowledge.UpdateDocumentsStatus(ctx, req.DocumentId, int(v1.StatusIndexing))
	if err != nil {
		g.Log().Errorf(ctx, "Failed to update document status to indexing, documentId=%s, err=%v", req.DocumentId, err)
		// 即使更新状态失败，也继续执行索引流程
	}

	// Define Pipeline steps
	pipeline := []struct {
		name string
		fn   func(*indexContext) error
	}{
		{"Get document info", s.stepGetDocument},
		{"Clean old data", s.stepCleanOldData},
		{"Prepare file", s.stepPrepareFile},
		{"Parse and split document", s.stepParseDocument},
		{"Save chunks", s.stepSaveChunks},
		{"Vectorize and store", s.stepVectorizeAndStore},
		{"Update status", s.stepUpdateStatus},
	}

	// Execute Pipeline
	for _, step := range pipeline {
		if err := step.fn(idxCtx); err != nil {
			return errors.Newf(errors.ErrIndexingFailed, "%s failed: %v", step.name, err)
		}
	}

	return nil
}

// stepGetDocument Step 1: Get document information
func (s *DocumentIndexer) stepGetDocument(idxCtx *indexContext) error {
	doc, err := knowledge.GetDocumentById(idxCtx.ctx, idxCtx.documentId)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to get document info, documentId=%s, err=%v", idxCtx.documentId, err)
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}
	idxCtx.doc = doc
	idxCtx.collectionName = doc.CollectionName
	return nil
}

// stepCleanOldData Step 2: Clean old document data
func (s *DocumentIndexer) stepCleanOldData(idxCtx *indexContext) error {
	err := knowledge.DeleteDocumentDataOnly(idxCtx.ctx, idxCtx.documentId, s.VectorStore)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to delete old document data, documentId=%s, err=%v", idxCtx.documentId, err)
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}
	return nil
}

// stepPrepareFile Step 3: Prepare file
func (s *DocumentIndexer) stepPrepareFile(idxCtx *indexContext) error {
	storageType := file_store.GetStorageType()
	idxCtx.storageType = storageType

	if storageType == file_store.StorageTypeRustFS {
		// RustFS storage: Download file from RustFS to upload/knowledge_file/知识库id/文件名 and overwrite existing file
		// 使用 LocalFilePath，如果为空则构建路径
		localFilePath := idxCtx.doc.LocalFilePath
		if localFilePath == "" {
			// 构建目标路径：upload/knowledge_file/知识库id/文件名
			localFilePath = gfile.Join("upload", "knowledge_file", idxCtx.doc.KnowledgeId, idxCtx.doc.FileName)
		}

		// Download file from RustFS to local path, overwrite if exists
		rustfsConfig := file_store.GetRustfsConfig()
		err := file_store.DownloadFile(idxCtx.ctx, rustfsConfig.Client, idxCtx.doc.RustfsBucket, idxCtx.doc.RustfsLocation, localFilePath)
		if err != nil {
			g.Log().Errorf(idxCtx.ctx, "Failed to download file from RustFS, documentId=%s, bucket=%s, location=%s, err=%v",
				idxCtx.documentId, idxCtx.doc.RustfsBucket, idxCtx.doc.RustfsLocation, err)
			_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
			return errors.Newf(errors.ErrFileReadFailed, "failed to download file from RustFS: %v", err)
		}
		g.Log().Infof(idxCtx.ctx, "File downloaded and overwritten from RustFS to %s, documentId=%s", localFilePath, idxCtx.documentId)
		idxCtx.localFilePath = localFilePath
	} else {
		// Local storage: Directly use the local_file_path stored in database
		if idxCtx.doc.LocalFilePath == "" {
			g.Log().Errorf(idxCtx.ctx, "Local file path is empty, documentId=%s", idxCtx.documentId)
			_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
			return errors.Newf(errors.ErrFileReadFailed, "local file path is empty, documentId=%s", idxCtx.documentId)
		}
		idxCtx.localFilePath = idxCtx.doc.LocalFilePath
	}

	// Check if file exists
	if idxCtx.localFilePath == "" || !fileExists(idxCtx.localFilePath) {
		g.Log().Errorf(idxCtx.ctx, "File does not exist, documentId=%s, path=%s", idxCtx.documentId, idxCtx.localFilePath)
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return errors.Newf(errors.ErrFileReadFailed, "file does not exist, path=%s", idxCtx.localFilePath)
	}

	return nil
}

// stepParseDocument Step 4: Parse and split document using file_parse service
func (s *DocumentIndexer) stepParseDocument(idxCtx *indexContext) error {
	// Create file_parse loader
	fileParseLoader, err := NewFileParseLoader(idxCtx.ctx, idxCtx.chunkSize, idxCtx.overlapSize, idxCtx.separator)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to create file_parse loader, documentId=%s, err=%v", idxCtx.documentId, err)
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}

	// Load and parse document using file_parse service
	chunks, err := fileParseLoader.Load(idxCtx.ctx, idxCtx.localFilePath)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to parse document with file_parse service, documentId=%s, err=%v", idxCtx.documentId, err)
		// 所有解析错误都应该标记文档为失败状态，包括服务不可用、超时等
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))

		errMsg := err.Error()
		// 检查是否是 file_parse 服务未启动或超时的错误，提供更明确的错误信息
		if strings.Contains(errMsg, "file_parse server is not running") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "unreachable") {
			return errors.Newf(errors.ErrDocumentParseFailed, "file_parse service unavailable: %v", err)
		}
		return errors.Newf(errors.ErrDocumentParseFailed, "failed to parse document: %v", err)
	}

	idxCtx.chunks = chunks
	g.Log().Infof(idxCtx.ctx, "Document parsing and splitting completed, documentId=%s, chunk count=%d", idxCtx.documentId, len(chunks))
	return nil
}

// stepSaveChunks Step 5: Save chunks to database
func (s *DocumentIndexer) stepSaveChunks(idxCtx *indexContext) error {
	if len(idxCtx.chunks) == 0 {
		return nil
	}

	chunkEntities := make([]gormModel.KnowledgeChunks, len(idxCtx.chunks))
	for i, chunk := range idxCtx.chunks {
		chunkId := uuid.New().String()

		// 使用统一的文本清洗工具：ProfileDatabase 包含所有数据库安全特性
		normalizedContent, err := common.CleanString(chunk.Content, common.ProfileDatabase)
		if err != nil {
			g.Log().Errorf(idxCtx.ctx, "Failed to clean chunk content, documentId=%s, chunkIndex=%d, err=%v",
				idxCtx.documentId, i, err)
			_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
			return errors.Newf(errors.ErrIndexingFailed, "failed to clean chunk content: %v", err)
		}

		// 更新chunk内容为清洗后的内容
		chunk.Content = normalizedContent

		// 从 metadata 中提取 chunk_index，存储到 ext 字段
		var extData string
		if chunkIndex, ok := chunk.MetaData["chunk_index"].(int); ok {
			// 将 chunk_index 转换为 JSON 字符串存储
			extJSON, err := json.Marshal(map[string]interface{}{
				"chunk_index": chunkIndex,
			})
			if err == nil {
				extData = string(extJSON)
			}
		}

		chunkEntities[i] = gormModel.KnowledgeChunks{
			ID:             chunkId,
			KnowledgeDocID: idxCtx.documentId,
			Content:        normalizedContent, // 使用清洗后的内容
			Ext:            extData,
			CollectionName: idxCtx.collectionName,
			Status:         int8(v1.ChunkStatusActive),
		}
		chunk.ID = chunkId
	}

	err := knowledge.SaveChunksData(idxCtx.ctx, idxCtx.documentId, chunkEntities)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to save chunks to database, documentId=%s, err=%v", idxCtx.documentId, err)
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return errors.Newf(errors.ErrIndexingFailed, "failed to save chunks to database: %v", err)
	}

	return nil
}

// stepVectorizeAndStore Step 7: Vectorize and store
func (s *DocumentIndexer) stepVectorizeAndStore(idxCtx *indexContext) error {
	// 从 Registry 获取 embedding 模型信息
	modelConfig := model.Registry.GetEmbeddingModel(idxCtx.modelID)
	if modelConfig == nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to get embedding model, documentId=%s, modelID=%s", idxCtx.documentId, idxCtx.modelID)
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return errors.Newf(errors.ErrModelNotFound, "embedding model not found in registry: %s", idxCtx.modelID)
	}

	// 创建动态配置，使用从 Registry 获取的模型信息覆盖静态配置
	dynamicConfig := &config.IndexerConfig{
		VectorStore:    s.Config.VectorStore,
		Database:       s.Config.Database,
		APIKey:         modelConfig.APIKey,  // 使用动态模型的 APIKey
		BaseURL:        modelConfig.BaseURL, // 使用动态模型的 BaseURL
		EmbeddingModel: modelConfig.Name,    // 使用动态模型的名称
		Dim:            modelConfig.Dimension,
		MetricType:     s.Config.MetricType,
	}

	g.Log().Infof(idxCtx.ctx, "Using dynamic embedding model, documentId=%s, modelID=%s, modelName=%s",
		idxCtx.documentId, idxCtx.modelID, modelConfig.Name)

	// 使用动态配置创建 vector embedder，传入模型配置和config dim
	embedder, err := NewVectorStoreEmbedder(idxCtx.ctx, dynamicConfig, s.VectorStore)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to create vector embedder, documentId=%s, err=%v", idxCtx.documentId, err)
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return errors.Newf(errors.ErrEmbeddingFailed, "failed to create vector embedder: %v", err)
	}

	// Set context, pass necessary information
	ctx := context.WithValue(idxCtx.ctx, vector_store.DocumentId, idxCtx.documentId)
	if idxCtx.doc.KnowledgeId != "" {
		ctx = context.WithValue(ctx, vector_store.KnowledgeId, idxCtx.doc.KnowledgeId)
	}

	// Use embedder to vectorize and store to vector database
	chunkIds, err := embedder.EmbedAndStore(ctx, idxCtx.collectionName, idxCtx.chunks)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to vectorize and store, documentId=%s, err=%v", idxCtx.documentId, err)
		_ = knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return errors.Newf(errors.ErrVectorInsert, "failed to vectorize and store: %v", err)
	}

	g.Log().Infof(idxCtx.ctx, "Vectorization completed, documentId=%s, collectionName=%s, chunks count=%d, successfully stored=%d",
		idxCtx.documentId, idxCtx.collectionName, len(idxCtx.chunks), len(chunkIds))

	return nil
}

// stepUpdateStatus Step 8: Update document status
func (s *DocumentIndexer) stepUpdateStatus(idxCtx *indexContext) error {
	err := knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusActive))
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to update document status, documentId=%s, err=%v", idxCtx.documentId, err)
		return err
	}
	return nil
}

// fileExists Check if file exists
func fileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}
