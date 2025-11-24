package vector_store

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/core/common"
	milvusModel "github.com/Malowking/kbgo/internal/model/milvus"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// MilvusStore Milvus向量数据库实现
type MilvusStore struct {
	client   *milvusclient.Client
	database string
}

// NewMilvusStore 创建Milvus向量存储实例
func NewMilvusStore(config *VectorStoreConfig) (*MilvusStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	client, ok := config.Client.(*milvusclient.Client)
	if !ok {
		return nil, fmt.Errorf("client must be *milvusclient.Client")
	}

	if config.Database == "" {
		return nil, fmt.Errorf("database name cannot be empty")
	}

	return &MilvusStore{
		client:   client,
		database: config.Database,
	}, nil
}

// CreateDatabaseIfNotExists 创建数据库（如果不存在）
func (m *MilvusStore) CreateDatabaseIfNotExists(ctx context.Context) error {
	dbNames, err := m.client.ListDatabase(ctx, milvusclient.NewListDatabaseOption())
	if err != nil {
		return fmt.Errorf("failed to list databases: %w", err)
	}

	for _, name := range dbNames {
		if strings.EqualFold(name, m.database) {
			g.Log().Infof(ctx, "Database '%s' already exists, skipping creation", m.database)
			return nil
		}
	}

	// 数据库不存在，创建
	err = m.client.CreateDatabase(ctx, milvusclient.NewCreateDatabaseOption(m.database))
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	g.Log().Infof(ctx, "Database '%s' created successfully", m.database)
	return nil
}

// CreateCollection 创建集合
func (m *MilvusStore) CreateCollection(ctx context.Context, collectionName string) error {
	// 使用标准 text collection schema
	schema := &entity.Schema{
		CollectionName: collectionName,
		Description:    "存储文档分片及其向量",
		AutoID:         false,
		Fields:         milvusModel.GetStandardCollectionFields(),
	}

	// 创建文档片段集合，并设置vector为索引
	err := m.client.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(collectionName, schema).WithIndexOptions(
		milvusclient.NewCreateIndexOption(collectionName, "vector", index.NewHNSWIndex(entity.L2, 64, 128))))
	if err != nil {
		return fmt.Errorf("failed to create Milvus collection: %w", err)
	}

	// Load collection into memory
	_, err = m.client.LoadCollection(ctx, milvusclient.NewLoadCollectionOption(collectionName))
	if err != nil {
		return fmt.Errorf("failed to load Milvus collection: %w", err)
	}

	g.Log().Infof(ctx, "Collection '%s' created, index built and loaded", collectionName)
	return nil
}

// CollectionExists 检查集合是否存在
func (m *MilvusStore) CollectionExists(ctx context.Context, collectionName string) (bool, error) {
	has, err := m.client.HasCollection(ctx, milvusclient.NewHasCollectionOption(collectionName))
	if err != nil {
		return false, fmt.Errorf("failed to check if collection exists: %w", err)
	}
	return has, nil
}

// DeleteCollection 删除集合
func (m *MilvusStore) DeleteCollection(ctx context.Context, collectionName string) error {
	err := m.client.DropCollection(ctx, milvusclient.NewDropCollectionOption(collectionName))
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}
	g.Log().Infof(ctx, "Collection '%s' deleted", collectionName)
	return nil
}

// InsertVectors 插入向量数据
func (m *MilvusStore) InsertVectors(ctx context.Context, collectionName string, chunks []*schema.Document, vectors [][]float64) ([]string, error) {
	if len(chunks) != len(vectors) {
		return nil, fmt.Errorf("chunks and vectors length mismatch: %d vs %d", len(chunks), len(vectors))
	}

	ids := make([]string, len(chunks))
	texts := make([]string, len(chunks))
	vectorsFloat32 := make([][]float32, len(chunks))
	documentIds := make([]string, len(chunks))
	metadataList := make([][]byte, len(chunks))

	// 从上下文中提取knowledge_id
	var knowledgeId string
	if value, ok := ctx.Value(common.KnowledgeId).(string); ok {
		knowledgeId = value
	}

	// 从上下文中提取document_id
	var contextDocumentId string
	if value, ok := ctx.Value(common.DocumentId).(string); ok {
		contextDocumentId = value
	}

	for idx, chunk := range chunks {
		// 生成chunk ID（如果不存在）
		if len(chunk.ID) == 0 {
			chunk.ID = uuid.New().String()
		}
		ids[idx] = chunk.ID

		// 截断文本（如果需要）
		texts[idx] = truncateString(chunk.Content, 65535)

		// 转换向量为float32
		vectorsFloat32[idx] = float64ToFloat32(vectors[idx])

		// 设置document_id
		var docID string
		if contextDocumentId != "" {
			docID = contextDocumentId
		} else {
			return nil, fmt.Errorf("document_id not found in context for chunk %s", chunk.ID)
		}
		documentIds[idx] = docID

		// 构建metadata
		metaCopy := make(map[string]any)
		if chunk.MetaData != nil {
			for k, v := range chunk.MetaData {
				metaCopy[k] = v
			}
		}

		// 添加knowledge_id到metadata
		if knowledgeId != "" {
			metaCopy[common.KnowledgeId] = knowledgeId
		}

		// 序列化metadata
		metaBytes, err := marshalMetadata(metaCopy)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataList[idx] = metaBytes
	}

	// 创建列数据
	columns := []column.Column{
		column.NewColumnVarChar("id", ids),
		column.NewColumnVarChar("text", texts),
		column.NewColumnFloatVector("vector", 1024, vectorsFloat32),
		column.NewColumnVarChar("document_id", documentIds),
		column.NewColumnJSONBytes("metadata", metadataList),
	}

	// 插入数据
	insertOpt := milvusclient.NewColumnBasedInsertOption(collectionName, columns...)
	result, err := m.client.Insert(ctx, insertOpt)
	if err != nil {
		return nil, fmt.Errorf("failed to insert vectors: %w", err)
	}

	g.Log().Infof(ctx, "Successfully inserted %d vectors into collection '%s'", result.InsertCount, collectionName)
	return ids, nil
}

// DeleteByDocumentID 根据文档ID删除所有相关chunks
func (m *MilvusStore) DeleteByDocumentID(ctx context.Context, collectionName string, documentID string) error {
	filterExpr := fmt.Sprintf(`document_id == "%s"`, documentID)

	g.Log().Infof(ctx, "Deleting all chunks of document %s from collection %s", documentID, collectionName)

	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filterExpr)
	result, err := m.client.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("failed to delete document %s: %w", documentID, err)
	}

	g.Log().Infof(ctx, "Delete operation completed for document %s, affected rows: %d", documentID, result.DeleteCount)

	if result.DeleteCount == 0 {
		g.Log().Infof(ctx, "Warning: No chunks were deleted for document_id=%s", documentID)
	}

	return nil
}

// DeleteByChunkID 根据chunkID删除单个chunk
func (m *MilvusStore) DeleteByChunkID(ctx context.Context, collectionName string, chunkID string) error {
	filterExpr := fmt.Sprintf(`id == "%s"`, chunkID)

	g.Log().Infof(ctx, "Deleting chunk %s from collection %s", chunkID, collectionName)

	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filterExpr)
	result, err := m.client.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("failed to delete chunk %s: %w", chunkID, err)
	}

	g.Log().Infof(ctx, "Delete operation completed for chunk %s, affected rows: %d", chunkID, result.DeleteCount)

	if result.DeleteCount == 0 {
		g.Log().Infof(ctx, "Warning: No chunk was deleted for id=%s", chunkID)
	}

	return nil
}

// Helper functions

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

func float64ToFloat32(f64 []float64) []float32 {
	f32 := make([]float32, len(f64))
	for i, v := range f64 {
		f32[i] = float32(v)
	}
	return f32
}

func marshalMetadata(metadata map[string]any) ([]byte, error) {
	if len(metadata) == 0 {
		return []byte("{}"), nil
	}
	return json.Marshal(metadata)
}
