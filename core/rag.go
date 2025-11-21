package core

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/indexer"
	"github.com/Malowking/kbgo/core/retriever"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// Rag represents the core RAG (Retrieval-Augmented Generation) service
// It provides methods to build indexers and retrievers on-demand rather than
// pre-creating and storing them in the struct, which is more resource-efficient
type Rag struct {
	Client *milvusclient.Client
	cm     model.BaseChatModel
	conf   *config.Config
}

// BuildIndexer creates an indexer for the specified collection
// collectionName: the Milvus collection name
func (r *Rag) BuildIndexer(ctx context.Context, collectionName string, chunkSize, overlapSize int) (compose.Runnable[any, []string], error) {
	return indexer.BuildIndexer(ctx, r.conf, collectionName, chunkSize, overlapSize)
}

// BuildIndexerWithSeparator creates an indexer for the specified collection with custom separator
// collectionName: the Milvus collection name
// separator: custom separator for document splitting
func (r *Rag) BuildIndexerWithSeparator(ctx context.Context, collectionName string, chunkSize, overlapSize int, separator string) (compose.Runnable[any, []string], error) {
	return indexer.BuildIndexerWithSeparator(ctx, r.conf, collectionName, chunkSize, overlapSize, separator)
}

// BuildIndexerAsync creates an async indexer for the specified collection
// collectionName: the Milvus collection name
//func (r *Rag) BuildIndexerAsync(ctx context.Context, collectionName string) (compose.Runnable[[]*schema.Document, []string], error) {
//	return indexer.BuildIndexerAsync(ctx, r.conf, collectionName)
//}

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
	// 确保milvus database存在
	err := common.CreateDatabaseIfNotExists(ctx, conf.Client, conf.Database)
	if err != nil {
		return nil, err
	}
	cm, err := common.GetChatModel(ctx, nil)
	if err != nil {
		g.Log().Error(ctx, "GetChatModel failed, err=%v", err)
		return nil, err
	}

	// 不再在 init 时创建 indexer 和 retriever
	// 而是在需要时通过 BuildIndexer/BuildRetriever 方法动态创建
	return &Rag{
		Client: conf.Client,
		conf:   conf,
		cm:     cm,
	}, nil
}
