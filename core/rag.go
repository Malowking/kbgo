package core

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/indexer/vector_store"
	"github.com/Malowking/kbgo/core/retriever"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// Rag represents the core RAG (Retrieval-Augmented Generation) service
// It provides methods to build retrievers for RAG functionality
type Rag struct {
	Client *milvusclient.Client
	cm     model.BaseChatModel
	conf   *config.Config
}

// BuildRetriever creates a retriever for the specified collection
// collectionName: the Milvus collection name
func (r *Rag) BuildRetriever(ctx context.Context, collectionName string) (compose.Runnable[string, []*schema.Document], error) {
	return retriever.BuildRetriever(ctx, r.conf, collectionName)
}

// GetConfig returns the configuration
func (r *Rag) GetConfig() *config.Config {
	return r.conf
}

// GetChatModel returns the chat model
func (r *Rag) GetChatModel() model.BaseChatModel {
	return r.cm
}

// GetClient returns the Milvus client
func (r *Rag) GetClient() *milvusclient.Client {
	return r.Client
}

func New(ctx context.Context, conf *config.Config) (*Rag, error) {
	if len(conf.Database) == 0 {
		return nil, fmt.Errorf("Database is empty")
	}

	// 创建向量存储配置并确保数据库存在
	vectorStoreConfig := &vector_store.VectorStoreConfig{
		Type:     vector_store.VectorStoreTypeMilvus,
		Client:   conf.Client,
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
		Client: conf.Client,
		conf:   conf,
		cm:     cm,
	}, nil
}
