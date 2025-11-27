package core

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/cloudwego/eino/components/model"
	"github.com/gogf/gf/v2/frame/g"
)

// Rag represents the core RAG (Retrieval-Augmented Generation) service
// It provides methods to build retrievers for RAG functionality
type Rag struct {
	VectorStore vector_store.VectorStore
	cm          model.BaseChatModel
	conf        *config.Config
}

// GetConfig returns the configuration
func (r *Rag) GetConfig() *config.Config {
	return r.conf
}

// GetRetrieverConfig returns retriever-specific configuration
func (r *Rag) GetRetrieverConfig() *config.RetrieverConfig {
	return &config.RetrieverConfig{
		VectorStore:    r.VectorStore,
		MetricType:     r.conf.MetricType,
		APIKey:         r.conf.APIKey,
		BaseURL:        r.conf.BaseURL,
		EmbeddingModel: r.conf.EmbeddingModel,
	}
}

// GetChatModel returns the chat model
func (r *Rag) GetChatModel() model.BaseChatModel {
	return r.cm
}

func New(ctx context.Context, conf *config.Config) (*Rag, error) {
	if len(conf.Database) == 0 {
		return nil, fmt.Errorf("Database is empty")
	}

	// 创建向量存储配置并确保数据库存在
	vectorStoreConfig := &vector_store.VectorStoreConfig{
		Type:     vector_store.VectorStoreTypeMilvus,
		Client:   conf.VectorStore,
		Database: conf.Database,
	}

	// 使用向量存储来确保数据库存在
	vs, err := vector_store.NewVectorStore(vectorStoreConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create vector store: %w", err)
	}

	err = vs.CreateDatabaseIfNotExists(ctx)
	if err != nil {
		return nil, err
	}

	cm, err := common.GetChatModel(ctx, nil)
	if err != nil {
		g.Log().Error(ctx, "GetChatModel failed, err=%v", err)
		return nil, err
	}

	// Rag 服务专注于检索功能，不再提供 indexer 方法
	// 文档索引应该使用独立的 DocumentIndexService
	return &Rag{
		VectorStore: conf.VectorStore,
		conf:        conf,
		cm:          cm,
	}, nil
}
