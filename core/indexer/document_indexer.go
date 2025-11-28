package indexer

import (
	"context"
	"fmt"
	"os"
	"sync"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/file_store"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/cloudwego/eino/schema"
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

// BatchDocumentIndex Batch document indexing processing (asynchronous operation)
func (s *DocumentIndexer) BatchDocumentIndex(ctx context.Context, req *BatchIndexReq) error {
	// 使用 WaitGroup 管理 goroutines
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 5) // 限制并发数为5

	for _, documentId := range req.DocumentIds {
		wg.Add(1)
		go func(docId string) {
			defer wg.Done()
			semaphore <- struct{}{} // Acquire semaphore
			defer func() {
				<-semaphore // 释放信号量
				if e := recover(); e != nil {
					g.Log().Errorf(ctx, "Document indexing exception, documentId=%s, err=%v", docId, e)
					knowledge.UpdateDocumentsStatus(ctx, docId, int(v1.StatusFailed))
				}
			}()

			indexReq := &IndexReq{
				ModelID:     req.ModelID,
				DocumentId:  docId,
				ChunkSize:   req.ChunkSize,
				OverlapSize: req.OverlapSize,
				Separator:   req.Separator,
			}

			err := s.DocumentIndex(ctx, indexReq)
			if err != nil {
				g.Log().Errorf(ctx, "Document indexing failed, documentId=%s, err=%v", docId, err)
			} else {
				g.Log().Infof(ctx, "Document indexed successfully, documentId=%s", docId)
			}
		}(documentId)
	}

	// Wait for all goroutines to complete
	go func() {
		wg.Wait()
		g.Log().Infof(ctx, "Batch document indexing completed, document count: %d", len(req.DocumentIds))
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
		{"Load document", s.stepLoadDocument},
		{"Split document", s.stepTransformDocument},
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

// stepLoadDocument Step 4: Load document
func (s *DocumentIndexer) stepLoadDocument(idxCtx *indexContext) error {
	// Create document loader
	docLoader, err := NewStandardDocumentLoader(idxCtx.ctx)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to create document loader, documentId=%s, err=%v", idxCtx.documentId, err)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}

	// Load document (returns []*schema.Document)
	docs, err := docLoader.Load(idxCtx.ctx, idxCtx.localFilePath)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to load document, documentId=%s, err=%v", idxCtx.documentId, err)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}

	// Save loaded documents to context for next step
	idxCtx.chunks = docs
	return nil
}

// stepTransformDocument Step 5: Split document
func (s *DocumentIndexer) stepTransformDocument(idxCtx *indexContext) error {
	// Create document transformer
	docTransformer, err := NewStandardDocumentTransformer(idxCtx.ctx, idxCtx.chunkSize, idxCtx.overlapSize, idxCtx.separator)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to create document transformer, documentId=%s, err=%v", idxCtx.documentId, err)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}

	// Split document
	chunks, err := docTransformer.Transform(idxCtx.ctx, idxCtx.chunks)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to split document, documentId=%s, err=%v", idxCtx.documentId, err)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusFailed))
		return err
	}

	idxCtx.chunks = chunks
	g.Log().Infof(idxCtx.ctx, "Document splitting completed, documentId=%s, chunk count=%d", idxCtx.documentId, len(chunks))
	return nil
}

// stepSaveChunks Step 6: Save chunks to database
func (s *DocumentIndexer) stepSaveChunks(idxCtx *indexContext) error {
	if len(idxCtx.chunks) == 0 {
		return nil
	}

	chunkEntities := make([]entity.KnowledgeChunks, len(idxCtx.chunks))
	for i, chunk := range idxCtx.chunks {
		chunkId := uuid.New().String()
		chunkEntities[i] = entity.KnowledgeChunks{
			Id:             chunkId,
			KnowledgeDocId: idxCtx.documentId,
			Content:        chunk.Content,
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
	}

	g.Log().Infof(idxCtx.ctx, "Using dynamic embedding model, documentId=%s, modelID=%s, modelName=%s",
		idxCtx.documentId, idxCtx.modelID, modelConfig.Name)

	// 使用动态配置创建 vector embedder
	embedder, err := NewVectorStoreEmbedder(idxCtx.ctx, dynamicConfig, s.VectorStore)
	if err != nil {
		g.Log().Errorf(idxCtx.ctx, "Failed to create vector embedder, documentId=%s, err=%v", idxCtx.documentId, err)
		knowledge.UpdateDocumentsStatus(idxCtx.ctx, idxCtx.documentId, int(v1.StatusIndexing))
		return fmt.Errorf("Failed to create vector embedder: %w", err)
	}

	// Set context, pass necessary information
	ctx := context.WithValue(idxCtx.ctx, common.DocumentId, idxCtx.documentId)
	if idxCtx.doc.KnowledgeId != "" {
		ctx = context.WithValue(ctx, common.KnowledgeId, idxCtx.doc.KnowledgeId)
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
