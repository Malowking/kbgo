package retriever

import (
	"context"

	"encoding/json"
	"github.com/Malowking/kbgo/core/errors"
	"sort"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/config"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/core/retriever"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/Malowking/kbgo/internal/service"
	"github.com/Malowking/kbgo/pkg/schema"
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

	// 从数据库的 model 表中读取默认的 embedding 和 rerank 模型
	// 获取第一个启用的 embedding 模型
	embeddingModels := model.Registry.GetByType(model.ModelTypeEmbedding)
	var embeddingAPIKey, embeddingBaseURL, embeddingModel string
	if len(embeddingModels) > 0 {
		embeddingAPIKey = embeddingModels[0].APIKey
		embeddingBaseURL = embeddingModels[0].BaseURL
		embeddingModel = embeddingModels[0].Name
		g.Log().Infof(ctx, "Using default embedding model from database: %s (ID: %s)", embeddingModel, embeddingModels[0].ModelID)
	} else {
		g.Log().Warning(ctx, "No embedding model found in database, embedding config will be empty")
	}

	// 获取第一个启用的 rerank 模型
	rerankModels := model.Registry.GetByType(model.ModelTypeReranker)
	var rerankAPIKey, rerankBaseURL, rerankModel string
	if len(rerankModels) > 0 {
		rerankAPIKey = rerankModels[0].APIKey
		rerankBaseURL = rerankModels[0].BaseURL
		rerankModel = rerankModels[0].Name
		g.Log().Infof(ctx, "Using default rerank model from database: %s (ID: %s)", rerankModel, rerankModels[0].ModelID)
	} else {
		g.Log().Warning(ctx, "No rerank model found in database, rerank config will be empty")
	}

	// 初始化 retrieverConfig，使用从数据库读取的模型配置
	retrieverConfig = &config.RetrieverConfig{
		RetrieverConfigBase: config.RetrieverConfigBase{
			MetricType:      g.Cfg().MustGet(ctx, "milvus.metricType", "COSINE").String(),
			APIKey:          embeddingAPIKey,
			BaseURL:         embeddingBaseURL,
			EmbeddingModel:  embeddingModel,
			RerankAPIKey:    rerankAPIKey,
			RerankBaseURL:   rerankBaseURL,
			RerankModel:     rerankModel,
			EnableRewrite:   g.Cfg().MustGet(ctx, "retriever.enableRewrite", false).Bool(),
			RewriteAttempts: g.Cfg().MustGet(ctx, "retriever.rewriteAttempts", 3).Int(),
			RetrieveMode:    g.Cfg().MustGet(ctx, "retriever.retrieveMode", "rerank").String(),
			TopK:            g.Cfg().MustGet(ctx, "retriever.topK", 5).Int(),
			Score:           g.Cfg().MustGet(ctx, "retriever.score", 0.2).Float64(),
		},
		VectorStore: vectorStore,
	}
}

// GetRetrieverConfig 获取 RetrieverConfig
func GetRetrieverConfig() *config.RetrieverConfig {
	return retrieverConfig
}

// ProcessRetrieval 处理检索请求
func ProcessRetrieval(ctx context.Context, req *v1.RetrieverReq) (*v1.RetrieverRes, error) {
	g.Log().Infof(ctx, "retrieveReq: %v, EmbeddingModelID: %v, RerankModelID: %v, EnableRewrite: %v, RewriteAttempts: %v, RetrieveMode: %v",
		req, req.EmbeddingModelID, req.RerankModelID, req.EnableRewrite, req.RewriteAttempts, req.RetrieveMode)

	// 如果没有提供 embedding_model_id，则从知识库获取绑定的模型
	embeddingModelID := req.EmbeddingModelID
	if embeddingModelID == "" {
		// 从数据库获取知识库信息
		var kb entity.KnowledgeBase
		err := dao.KnowledgeBase.Ctx(ctx).WherePri(req.KnowledgeId).Scan(&kb)
		if err != nil {
			return nil, errors.Newf(errors.ErrKBNotFound, "failed to get knowledge base: %v", err)
		}
		if kb.Id == "" {
			return nil, errors.Newf(errors.ErrKBNotFound, "knowledge base not found: %s", req.KnowledgeId)
		}
		if kb.EmbeddingModelId == "" {
			return nil, errors.Newf(errors.ErrModelNotConfigured, "knowledge base %s has no embedding model bound", req.KnowledgeId)
		}
		embeddingModelID = kb.EmbeddingModelId
		g.Log().Infof(ctx, "Using knowledge base bound embedding model: %s", embeddingModelID)
	}

	// 从 Registry 获取 embedding 模型信息
	embeddingModelConfig := model.Registry.Get(embeddingModelID)
	if embeddingModelConfig == nil {
		return nil, errors.Newf(errors.ErrModelNotFound, "embedding model not found in registry: %s", embeddingModelID)
	}

	// 验证 embedding 模型类型
	if embeddingModelConfig.Type != model.ModelTypeEmbedding {
		return nil, errors.Newf(errors.ErrModelConfigInvalid, "model %s is not an embedding model, got type: %s", embeddingModelID, embeddingModelConfig.Type)
	}

	// 创建动态配置，使用从 Registry 获取的模型信息覆盖静态配置
	dynamicConfig := &config.RetrieverConfig{
		RetrieverConfigBase: config.RetrieverConfigBase{
			MetricType:      retrieverConfig.MetricType,
			APIKey:          embeddingModelConfig.APIKey,  // 使用动态 embedding 模型的 APIKey
			BaseURL:         embeddingModelConfig.BaseURL, // 使用动态 embedding 模型的 BaseURL
			EmbeddingModel:  embeddingModelConfig.Name,    // 使用动态 embedding 模型的名称
			RerankAPIKey:    retrieverConfig.RerankAPIKey, // 先使用静态配置的默认值
			RerankBaseURL:   retrieverConfig.RerankBaseURL,
			RerankModel:     retrieverConfig.RerankModel,
			EnableRewrite:   retrieverConfig.EnableRewrite,
			RewriteAttempts: retrieverConfig.RewriteAttempts,
			RetrieveMode:    retrieverConfig.RetrieveMode,
			TopK:            retrieverConfig.TopK,
			Score:           retrieverConfig.Score,
		},
		VectorStore: retrieverConfig.VectorStore,
	}

	// 如果提供了 RerankModelID，则从 Registry 获取 rerank 模型配置
	if req.RerankModelID != "" {
		rerankModelConfig := model.Registry.Get(req.RerankModelID)
		if rerankModelConfig == nil {
			return nil, errors.Newf(errors.ErrModelNotFound, "rerank model not found in registry: %s", req.RerankModelID)
		}

		// 验证 rerank 模型类型
		if rerankModelConfig.Type != model.ModelTypeReranker {
			return nil, errors.Newf(errors.ErrModelConfigInvalid, "model %s is not a reranker model, got type: %s", req.RerankModelID, rerankModelConfig.Type)
		}

		// 使用动态 rerank 模型配置
		dynamicConfig.RerankAPIKey = rerankModelConfig.APIKey
		dynamicConfig.RerankBaseURL = rerankModelConfig.BaseURL
		dynamicConfig.RerankModel = rerankModelConfig.Name

		g.Log().Infof(ctx, "Using dynamic rerank model: modelID=%s, modelName=%s", req.RerankModelID, rerankModelConfig.Name)
	}

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

		// 如果使用 rerank 或 rrf 模式，但没有提供 RerankModelID，返回错误
		if (req.RetrieveMode == "rerank" || req.RetrieveMode == "rrf") && req.RerankModelID == "" {
			return nil, errors.Newf(errors.ErrInvalidParameter, "rerank_model_id is required when retrieve_mode is %s", req.RetrieveMode)
		}
	}

	// EnableRewrite 相关的参数设置
	if req.EnableRewrite {
		retrieveReq.EnableRewrite = &req.EnableRewrite
		if req.RewriteAttempts != 0 {
			retrieveReq.RewriteAttempts = &req.RewriteAttempts
		}
	}

	// RerankWeight 参数传递
	if req.RerankWeight != nil {
		retrieveReq.RerankWeight = req.RerankWeight
	}

	// 使用动态配置调用 retriever
	msg, err := retriever.Retrieve(ctx, dynamicConfig, retrieveReq)
	if err != nil {
		return nil, err
	}

	// 处理元数据：将JSON字符串解析为map
	msg = processDocumentMetadata(msg)

	// 按分数降序排序
	sort.Slice(msg, func(i, j int) bool {
		return msg[i].Score > msg[j].Score
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
