package vector_store

import (
	"context"

	"github.com/cloudwego/eino/schema"
)

// VectorStoreType 向量数据库类型
type VectorStoreType string

const (
	VectorStoreTypeMilvus VectorStoreType = "milvus"
	// 未来可以扩展其他类型
	// VectorStoreTypeChroma VectorStoreType = "chroma"
	// VectorStoreTypeWeaviate VectorStoreType = "weaviate"
)

// VectorStoreConfig 向量数据库配置
type VectorStoreConfig struct {
	Type     VectorStoreType // 向量数据库类型
	Client   interface{}     // 客户端实例
	Database string          // 数据库名称
	// 可以根据需要添加其他配置项
	MetricType string            // 距离度量类型（如 L2, COSINE, IP）
	Extra      map[string]string // 额外配置
}

// VectorStore 向量数据库接口
type VectorStore interface {
	// CreateCollection 创建集合
	CreateCollection(ctx context.Context, collectionName string) error

	// CollectionExists 检查集合是否存在
	CollectionExists(ctx context.Context, collectionName string) (bool, error)

	// DeleteCollection 删除集合
	DeleteCollection(ctx context.Context, collectionName string) error

	// InsertVectors 插入向量数据
	InsertVectors(ctx context.Context, collectionName string, chunks []*schema.Document, vectors [][]float64) ([]string, error)

	// DeleteByDocumentID 根据文档ID删除所有相关chunks
	DeleteByDocumentID(ctx context.Context, collectionName string, documentID string) error

	// DeleteByChunkID 根据chunkID删除单个chunk
	DeleteByChunkID(ctx context.Context, collectionName string, chunkID string) error

	// CreateDatabaseIfNotExists 创建数据库（如果不存在）
	CreateDatabaseIfNotExists(ctx context.Context) error
}
