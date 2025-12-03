package vector_store

import (
	"fmt"
)

// NewVectorStore 根据配置创建向量存储实例
func NewVectorStore(config *VectorStoreConfig) (VectorStore, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	switch config.Type {
	case VectorStoreTypeMilvus:
		return NewMilvusStore(config)
	case VectorStoreTypePostgreSQL:
		return NewPostgresStore(config)
	default:
		return nil, fmt.Errorf("unsupported vector store type: %s", config.Type)
	}
}
