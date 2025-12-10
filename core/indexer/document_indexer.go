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
	"github.com/Malowking/kbgo/core/file_store"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
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
	doc            entity.KnowledgeDocuments
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

// BatchDocumentIndex Batch document indexing processing (asynchronous operation)
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
			} else {
				g.Log().Infof(ctx, "Document indexed successfully, documentId=%s", documentId)
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

// DocumentIndex Unified document indexing processing (using Pipeline pattern)
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
		g.Log().Debugf(ctx, "Executing step: %s, documentId=%s", step.name, req.DocumentId)
		if err := step.fn(idxCtx); err != nil {
			return fmt.Errorf("%s failed: %w", step.name, err)
		}
	}

	return nil
}

// stepGetDocument Step 1: Get document information
func (s *DocumentIndexer) stepGetDocument(idxCtx *indexContext) error {
	doc, err := knowledge.GetDocumentById(idxCtx.ctx, idxCtx.documentId)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to get document info, documentId=%s, err=%v", idxCtx.documentId, err)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
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
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}
	return nil
}

// stepPrepareFile Step 3: Prepare file (handle storage type and file path)
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
			knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
			return err
		}
		g.Log().Infof(idxCtx.ctx, "File downloaded and overwritten from RustFS to %s, documentId=%s", localFilePath, idxCtx.documentId)
		idxCtx.localFilePath = localFilePath
	} else {
		// Local storage: Directly use the local_file_path stored in database (relative path)
		if idxCtx.doc.LocalFilePath == "" {
			err := fmt.Errorf("Local file path is empty, documentId=%s", idxCtx.documentId)
			g.Log().Errorf(idxCtx.ctx, "Local file path is empty, documentId=%s", idxCtx.documentId)
			knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
			return err
		}
		idxCtx.localFilePath = idxCtx.doc.LocalFilePath
	}

	// Check if file exists
	if idxCtx.localFilePath == "" || !fileExists(idxCtx.localFilePath) {
		err := fmt.Errorf("File does not exist, path=%s", idxCtx.localFilePath)
		g.Log().Errorf(idxCtx.ctx, "File does not exist, documentId=%s, path=%s", idxCtx.documentId, idxCtx.localFilePath)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}

	return nil
}

// stepParseDocument Step 4: Parse and split document using file_parse service
func (s *DocumentIndexer) stepParseDocument(idxCtx *indexContext) error {
	// Create file_parse loader
	fileParseLoader, err := NewFileParseLoader(idxCtx.ctx, idxCtx.chunkSize, idxCtx.overlapSize, idxCtx.separator)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to create file_parse loader, documentId=%s, err=%v", idxCtx.documentId, err)
		// 不修改数据库状态，直接返回错误
		return err
	}

	// Load and parse document using file_parse service
	chunks, err := fileParseLoader.Load(idxCtx.ctx, idxCtx.localFilePath)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to parse document with file_parse service, documentId=%s, err=%v", idxCtx.documentId, err)
		errMsg := err.Error()
		// 检查是否是 file_parse 服务未启动或超时的错误
		if strings.Contains(errMsg, "file_parse server is not running") ||
			strings.Contains(errMsg, "timeout") ||
			strings.Contains(errMsg, "unreachable") {
			// 对于服务未启动或超时错误，不修改状态，直接返回
			return fmt.Errorf("file_parse service unavailable: %w", err)
		}
		// 其他错误（如文件解析失败），标记为失败
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
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

	chunkEntities := make([]entity.KnowledgeChunks, len(idxCtx.chunks))
	for i, chunk := range idxCtx.chunks {
		chunkId := uuid.New().String()

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

		chunkEntities[i] = entity.KnowledgeChunks{
			Id:             chunkId,
			KnowledgeDocId: idxCtx.documentId,
			Content:        chunk.Content,
			Ext:            extData,
			CollectionName: idxCtx.collectionName,
			Status:         int(v1.StatusPending),
		}
		chunk.ID = chunkId
	}

	err := knowledge.SaveChunksData(idxCtx.ctx, idxCtx.documentId, chunkEntities)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to save chunks to database, documentId=%s, err=%v", idxCtx.documentId, err)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusIndexing))
		return fmt.Errorf("Failed to save chunks to database: %w", err)
	}

	return nil
}

// stepVectorizeAndStore Step 7: Vectorize and store
func (s *DocumentIndexer) stepVectorizeAndStore(idxCtx *indexContext) error {
	// 从 Registry 获取 embedding 模型信息
	modelConfig := model.Registry.Get(idxCtx.modelID)
	if modelConfig == nil {
		err := fmt.Errorf("embedding model not found in registry: %s", idxCtx.modelID)
		g.Log().Errorf(idxCtx.ctx, "Failed to get embedding model, documentId=%s, modelID=%s", idxCtx.documentId, idxCtx.modelID)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}

	// 验证模型类型
	if modelConfig.Type != model.ModelTypeEmbedding {
		err := fmt.Errorf("model %s is not an embedding model, got type: %s", idxCtx.modelID, modelConfig.Type)
		g.Log().Errorf(idxCtx.ctx, "Invalid model type, documentId=%s, modelID=%s, type=%s",
			idxCtx.documentId, idxCtx.modelID, modelConfig.Type)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}

	// 创建动态配置，使用从 Registry 获取的模型信息覆盖静态配置
	dynamicConfig := &config.IndexerConfig{
		VectorStore:    s.Config.VectorStore,
		Database:       s.Config.Database,
		APIKey:         modelConfig.APIKey,  // 使用动态模型的 APIKey
		BaseURL:        modelConfig.BaseURL, // 使用动态模型的 BaseURL
		EmbeddingModel: modelConfig.Name,    // 使用动态模型的名称
		MetricType:     s.Config.MetricType,
		Dim:            s.Config.Dim, // 使用配置文件的 dim 作为fallback
	}

	g.Log().Infof(idxCtx.ctx, "Using dynamic embedding model, documentId=%s, modelID=%s, modelName=%s",
		idxCtx.documentId, idxCtx.modelID, modelConfig.Name)

	// 使用动态配置创建 vector embedder，传入模型配置和config dim
	embedder, err := NewVectorStoreEmbedder(idxCtx.ctx, dynamicConfig, s.VectorStore, modelConfig, s.Config.Dim)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to create vector embedder, documentId=%s, err=%v", idxCtx.documentId, err)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusIndexing))
		return fmt.Errorf("Failed to create vector embedder: %w", err)
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
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusIndexing))
		return fmt.Errorf("Failed to vectorize and store: %w", err)
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
