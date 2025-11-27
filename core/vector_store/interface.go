package vector_store

import (
	"context"

	"github.com/cloudwego/eino/components/retriever"
	"github.com/cloudwego/eino/schema"
	"github.com/milvus-io/milvus/client/v2/column"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// VectorStoreType 向量数据库类型
type VectorStoreType string

const (
	VectorStoreTypeMilvus VectorStoreType = "milvus"
	// 未来可以扩展其他类型
	// VectorStoreTypeChroma VectorStoreType = "chroma"
	// VectorStoreTypeWeaviate VectorStoreType = "weaviate"
)

// GeneralRetrieverConfig 通用检索配置接口
// 避免循环导入，使用接口来传递检索配置
type GeneralRetrieverConfig interface {
	GetTopK() int
	GetScore() float64
	GetEnableRewrite() bool
	GetRewriteAttempts() int
	GetRetrieveMode() string
}

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

	// GetClient 获取底层客户端实例
	GetClient() *milvusclient.Client

	// NewMilvusRetriever 创建Milvus检索器实例
	NewMilvusRetriever(ctx context.Context, conf interface{}, collectionName string) (retriever.Retriever, error)

	// ConvertSearchResultsToDocuments 将搜索结果转换为schema.Document格式
	// 并进行权限控制，过滤掉status != 1的chunks
	ConvertSearchResultsToDocuments(ctx context.Context, columns []column.Column, scores []float32) ([]*schema.Document, error)

	// VectorSearchOnly 仅使用向量检索的通用方法
	// 执行向量相似度搜索，去重，排序，并按分数过滤结果
	VectorSearchOnly(ctx context.Context, conf GeneralRetrieverConfig, query string, knowledgeId string, topK int, score float64) ([]*schema.Document, error)
}
