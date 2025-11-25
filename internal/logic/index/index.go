package index

import (
	"github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/indexer"
	"github.com/Malowking/kbgo/internal/service"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

var docIndexSvr *indexer.DocumentIndexer

func InitVectorDatabase() {
	ctx := gctx.New()

	// 确保共享服务已初始化
	if service.GetSharedConfig() == nil {
		if err := service.InitSharedServices(ctx); err != nil {
			g.Log().Fatalf(ctx, "Failed to init shared services: %v", err)
			return
		}
	}

	// 获取共享配置
	sharedConfig := service.GetSharedConfig()

	// 创建 Milvus 客户端（DocumentIndexService 专用）
	milvusAddress := g.Cfg().MustGet(ctx, "milvus.address", "").String()
	if milvusAddress == "" {
		g.Log().Fatalf(ctx, "Milvus configuration is missing: milvus.address is required")
		return
	}

	milvusClient, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: milvusAddress,
		DBName:  sharedConfig.Database,
	})
	if err != nil {
		g.Log().Fatalf(ctx, "Failed to create Milvus client for DocumentIndexService: %v", err)
		return
	}

	// 创建 DocumentIndexer 配置
	indexConfig := &config.Config{
		Client:         milvusClient,
		Database:       sharedConfig.Database,
		APIKey:         sharedConfig.APIKey,
		BaseURL:        sharedConfig.BaseURL,
		EmbeddingModel: sharedConfig.EmbeddingModel,
		ChatModel:      sharedConfig.ChatModel,
		MetricType:     sharedConfig.MetricType,
	}

	// 初始化 DocumentIndexer
	docIndexSvr, err = core.NewDocumentIndexer(ctx, indexConfig)
	if err != nil {
		g.Log().Fatalf(ctx, "Failed to create DocumentIndexService: %v", err)
		return
	}

	g.Log().Info(ctx, "DocumentIndexService initialized successfully")
}

// GetDocIndexSvr 获取文档索引服务实例
func GetDocIndexSvr() *indexer.DocumentIndexer {
	return docIndexSvr
}
