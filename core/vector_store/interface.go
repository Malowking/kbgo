package vector_store

import (
	"context"

	"github.com/Malowking/kbgo/pkg/schema"
)

// 向量存储字段名常量
const (
	FieldContent       = "text"
	FieldContentVector = "vector"
	FieldMetadata      = "metadata"
	KnowledgeId        = "knowledge_id"
	DocumentId         = "document_id"
)

// NL2SQL向量存储字段名常量
const (
	NL2SQLFieldEntityType   = "entity_type"
	NL2SQLFieldEntityId     = "entity_id"
	NL2SQLFieldDatasourceId = "datasource_id"
	NL2SQLFieldText         = "text"
	NL2SQLFieldVector       = "vector"
	NL2SQLFieldMetadata     = "metadata"
)

// Retriever interface for vector search and retrieval operations
type Retriever interface {
	// Retrieve performs vector search and returns matching documents
	Retrieve(ctx context.Context, query string, opts ...Option) ([]*schema.Document, error)

	// GetType returns the type of retriever
	GetType() string

	// IsCallbacksEnabled returns whether callbacks are enabled
	IsCallbacksEnabled() bool
}

// Option represents a functional option for retriever configuration
type Option func(*Options)

// Options contains configuration options for retrieval
type Options struct {
	TopK           *int
	ScoreThreshold *float64
	Filter         string
	Partition      string
}

// WithTopK sets the number of top results to return
func WithTopK(topK int) Option {
	return func(o *Options) {
		o.TopK = &topK
	}
}

// WithScoreThreshold sets the minimum score threshold
func WithScoreThreshold(threshold float64) Option {
	return func(o *Options) {
		o.ScoreThreshold = &threshold
	}
}

// WithFilter sets the filter expression for Milvus
func WithFilter(filter string) Option {
	return func(o *Options) {
		o.Filter = filter
	}
}

// WithPartition sets the partition for Milvus search
func WithPartition(partition string) Option {
	return func(o *Options) {
		o.Partition = partition
	}
}

// GetCommonOptions applies options and returns the resulting configuration
func GetCommonOptions(defaultOpts *Options, opts ...Option) *Options {
	if defaultOpts == nil {
		defaultOpts = &Options{}
	}
	result := *defaultOpts // copy
	for _, opt := range opts {
		opt(&result)
	}
	return &result
}

// VectorStoreType 向量数据库类型
type VectorStoreType string

const (
	VectorStoreTypeMilvus     VectorStoreType = "milvus"
	VectorStoreTypePostgreSQL VectorStoreType = "pgvector"
	// 未来可以扩展其他类型
	// VectorStoreTypeChroma VectorStoreType = "chroma"
	// VectorStoreTypeWeaviate VectorStoreType = "weaviate"
)

// GeneralRetrieverConfig 通用检索配置接口
type GeneralRetrieverConfig interface {
	GetTopK() int
	GetScore() float64
	GetEnableRewrite() bool
	GetRewriteAttempts() int
	GetRetrieveMode() string
}

// VectorStoreConfig 向量数据库配置
type VectorStoreConfig struct {
	Type       VectorStoreType   // 向量数据库类型
	Client     interface{}       // 客户端实例
	Database   string            // 数据库名称
	MetricType string            // 距离度量类型（如 L2, COSINE, IP）
	Extra      map[string]string // 额外配置
}

// VectorStore 向量数据库接口
type VectorStore interface {
	// CreateCollection 创建集合
	CreateCollection(ctx context.Context, collectionName string, dimension int) error

	// CollectionExists 检查集合是否存在
	CollectionExists(ctx context.Context, collectionName string) (bool, error)

	// DeleteCollection 删除集合
	DeleteCollection(ctx context.Context, collectionName string) error

	// InsertVectors 插入向量数据
	InsertVectors(ctx context.Context, collectionName string, chunks []*schema.Document, vectors [][]float32) ([]string, error)

	// DeleteByDocumentID 根据文档ID删除所有相关chunks
	DeleteByDocumentID(ctx context.Context, collectionName string, documentID string) error

	// DeleteByChunkID 根据chunkID删除单个chunk
	DeleteByChunkID(ctx context.Context, collectionName string, chunkID string) error

	// CreateDatabaseIfNotExists 创建数据库
	CreateDatabaseIfNotExists(ctx context.Context) error

	// GetClient 获取底层客户端实例
	GetClient() interface{}

	// NewRetriever 创建检索器实例
	NewRetriever(ctx context.Context, collectionName string) (Retriever, error)

	// VectorSearchOnly 仅使用向量检索的通用方法
	VectorSearchOnly(ctx context.Context, conf GeneralRetrieverConfig, query string, knowledgeId string, topK int, score float64) ([]*schema.Document, error)

	// VectorSearchOnlyNL2SQL NL2SQL专用的向量检索方法
	VectorSearchOnlyNL2SQL(ctx context.Context, query string, collectionName string, datasourceID string, topK int, score float64) ([]*schema.Document, error)

	// CreateNL2SQLCollection 创建NL2SQL专用的集合
	CreateNL2SQLCollection(ctx context.Context, collectionName string, dimension int) error

	// InsertNL2SQLVectors 插入NL2SQL向量数据
	InsertNL2SQLVectors(ctx context.Context, collectionName string, entities []*NL2SQLEntity, vectors [][]float32) ([]string, error)

	// DeleteNL2SQLByDatasourceID 根据数据源ID删除所有相关实体
	DeleteNL2SQLByDatasourceID(ctx context.Context, collectionName string, datasourceID string) error
}

// NL2SQLEntity represents a NL2SQL entity for vector storage
type NL2SQLEntity struct {
	ID           string                 `json:"id"`
	EntityType   string                 `json:"entity_type"`   // table, column, metric, relation
	EntityID     string                 `json:"entity_id"`     // UUID of the entity
	DatasourceID string                 `json:"datasource_id"` // Datasource ID
	Text         string                 `json:"text"`          // Text for embedding
	MetaData     map[string]interface{} `json:"metadata"`      // Additional metadata
}
