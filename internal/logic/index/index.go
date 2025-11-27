package index

import (
	"fmt"

	"github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/indexer"
	"github.com/Malowking/kbgo/internal/service"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

var (
	docIndexSvr *indexer.DocumentIndexer
	indexConfig *config.IndexerConfig
)

func InitDocumentIndexer() {
	ctx := gctx.New()

	vectorDBType := g.Cfg().MustGet(ctx, "vectordb.type", "milvus").String()
	Database := g.Cfg().MustGet(ctx, fmt.Sprintf("%s.database", vectorDBType)).String()
	APIKey := g.Cfg().MustGet(ctx, "embedding.apiKey").String()
	BaseURL := g.Cfg().MustGet(ctx, "embedding.baseURL").String()
	EmbeddingModel := g.Cfg().MustGet(ctx, "embedding.model").String()

	// 距离度量类型
	MetricType := g.Cfg().MustGet(ctx, "vectordb.metricType", "L2").String()

	// 初始化全局 IndexerConfig
	vectorStore, err := service.GetVectorStore()
	if err != nil {
		g.Log().Fatalf(ctx, "Failed to get vector store: %v", err)
		return
	}
	err = vectorStore.CreateDatabaseIfNotExists(ctx)
	if err != nil {
		g.Log().Fatalf(ctx, "Failed to create vector database: %v", err)
		return
	}

	indexConfig = &config.IndexerConfig{
		VectorStore:    vectorStore,
		Database:       Database,
		APIKey:         APIKey,
		BaseURL:        BaseURL,
		EmbeddingModel: EmbeddingModel,
		MetricType:     MetricType,
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
