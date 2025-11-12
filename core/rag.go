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

type Rag struct {
	//idxer      compose.Runnable[any, []string]
	//idxerAsync compose.Runnable[[]*schema.Document, []string]
	//rtrvr      compose.Runnable[string, []*schema.Document]
	//qaRtrvr    compose.Runnable[string, []*schema.Document]
	Client *milvusclient.Client
	cm     model.BaseChatModel

	//grader *grader.Grader // 暂时先弃用，使用 grader 会严重影响rag的速度
	conf *config.Config
}

// BuildIndexer creates an indexer for the specified collection
// collectionName: the Milvus collection name
func (r *Rag) BuildIndexer(ctx context.Context, collectionName string, chunkSize, overlapSize int) (compose.Runnable[any, []string], error) {
	return indexer.BuildIndexer(ctx, r.conf, collectionName, chunkSize, overlapSize)
}

// BuildIndexerAsync creates an async indexer for the specified collection
// collectionName: the Milvus collection name
func (r *Rag) BuildIndexerAsync(ctx context.Context, collectionName string) (compose.Runnable[[]*schema.Document, []string], error) {
	return indexer.BuildIndexerAsync(ctx, r.conf, collectionName)
}

// BuildRetriever creates a retriever for the specified collection
// collectionName: the Milvus collection name
func (r *Rag) BuildRetriever(ctx context.Context, collectionName string) (compose.Runnable[string, []*schema.Document], error) {
	return retriever.BuildRetriever(ctx, r.conf)
}

// GetConfig returns the configuration
func (r *Rag) GetConfig() *config.Config {
	return r.conf
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

// GetKnowledgeBaseList 获取知识库列表
// 从 Milvus 获取所有 text_ 开头的 collection，提取知识库 ID
func (x *Rag) GetKnowledgeBaseList(ctx context.Context) (list []string, err error) {
	// 列出指定 database 中的所有 collection
	listOpt := milvusclient.NewListCollectionOption()
	collections, err := x.Client.ListCollections(ctx, listOpt)
	if err != nil {
		g.Log().Errorf(ctx, "failed to list collections: %v", err)
		return nil, err
	}

	// 过滤出 text_ 开头的 collection，提取知识库 ID
	kbIDSet := make(map[string]bool)
	for _, collectionName := range collections {
		// 只处理 text_ 开头的 collection
		if len(collectionName) > 5 && collectionName[:5] == "text_" {
			// 提取知识库 ID（text_xxx -> xxx）
			kbID := collectionName[5:]
			kbIDSet[kbID] = true
		}
	}

	// 转换为列表
	list = make([]string, 0, len(kbIDSet))
	for kbID := range kbIDSet {
		list = append(list, kbID)
	}

	return list, nil
}
