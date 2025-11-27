package retriever

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/retriever"
	"github.com/Malowking/kbgo/internal/service"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

var retrieverConfig *config.RetrieverConfig

func InitRetrieverConfig() {
	ctx := gctx.New()
	vectorStore, err := service.GetVectorStore()
	if err != nil {
		g.Log().Fatalf(ctx, "Failed to get vector store: %v", err)
		return
	}
	// 初始化 retrieverConfig
	retrieverConfig = &config.RetrieverConfig{
		VectorStore:     vectorStore,
		MetricType:      g.Cfg().MustGet(ctx, "milvus.metricType", "COSINE").String(),
		APIKey:          g.Cfg().MustGet(ctx, "embedding.apiKey").String(),
		BaseURL:         g.Cfg().MustGet(ctx, "embedding.baseURL").String(),
		EmbeddingModel:  g.Cfg().MustGet(ctx, "embedding.model").String(),
		EnableRewrite:   g.Cfg().MustGet(ctx, "retriever.enableRewrite", false).Bool(),
		RewriteAttempts: g.Cfg().MustGet(ctx, "retriever.rewriteAttempts", 3).Int(),
		RetrieveMode:    g.Cfg().MustGet(ctx, "retriever.retrieveMode", "rerank").String(),
		TopK:            g.Cfg().MustGet(ctx, "retriever.topK", 5).Int(),
		Score:           g.Cfg().MustGet(ctx, "retriever.score", 0.2).Float64(),
	}
}

// GetRetrieverConfig 获取 RetrieverConfig
func GetRetrieverConfig() *config.RetrieverConfig {
	return retrieverConfig
}

// ProcessRetrieval 处理检索请求
func ProcessRetrieval(ctx context.Context, req *v1.RetrieverReq) (*v1.RetrieverRes, error) {
	g.Log().Infof(ctx, "retrieveReq: %v, EnableRewrite: %v, RewriteAttempts: %v, RetrieveMode: %v", req, req.EnableRewrite, req.RewriteAttempts, req.RetrieveMode)

	// 构建内部请求，只传递必需参数和显式指定的可选参数
	retrieveReq := &retriever.RetrieveReq{
		Query:       req.Question,
		KnowledgeId: req.KnowledgeId,
	}

	// 只有当请求中明确提供了参数时才覆盖配置默认值
	if req.TopK != 0 {
		retrieveReq.TopK = &req.TopK
	}
	if req.Score != 0 {
		retrieveReq.Score = &req.Score
	}

	// RetrieveMode 是独立的检索模式设置，不依赖于 EnableRewrite
	if req.RetrieveMode != "" {
		mode := retriever.RetrieveMode(req.RetrieveMode)
		retrieveReq.RetrieveMode = &mode
	}

	// EnableRewrite 相关的参数设置
	if req.EnableRewrite {
		retrieveReq.EnableRewrite = &req.EnableRewrite
		if req.RewriteAttempts != 0 {
			retrieveReq.RewriteAttempts = &req.RewriteAttempts
		}
	}

	// 直接获取retriever配置并调用retriever
	msg, err := retriever.Retrieve(ctx, retrieverConfig, retrieveReq)
	if err != nil {
		return nil, err
	}

	// 处理元数据：将JSON字符串解析为map
	msg = processDocumentMetadata(msg)

	// 按分数降序排序
	sort.Slice(msg, func(i, j int) bool {
		return msg[i].Score() > msg[j].Score()
	})

	return &v1.RetrieverRes{
		Document: msg,
	}, nil
}

// processDocumentMetadata 处理文档元数据，将JSON字符串解析为map
func processDocumentMetadata(documents []*schema.Document) []*schema.Document {
	for _, document := range documents {
		if document.MetaData != nil {
			if metadataVal, ok := document.MetaData["metadata"]; ok && metadataVal != nil {
				// 尝试将 metadata 从 JSON 字符串解析为 map
				if metadataStr, isString := metadataVal.(string); isString && metadataStr != "" {
					m := make(map[string]interface{})
					if err := json.Unmarshal([]byte(metadataStr), &m); err == nil {
						document.MetaData["metadata"] = m
					}
					// 如果解析失败，保持原值
				}
			}
		}
	}
	return documents
}
