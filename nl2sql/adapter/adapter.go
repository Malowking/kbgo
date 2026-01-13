package adapter

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// LLMAdapter LLM适配器,将ModelService适配为NL2SQL所需的LLMFunc接口
type LLMAdapter struct {
	modelConfig *model.ChatModelConfig
}

// NewLLMAdapter 创建LLM适配器
func NewLLMAdapter(modelConfig *model.ChatModelConfig) *LLMAdapter {
	return &LLMAdapter{
		modelConfig: modelConfig,
	}
}

// Call 调用LLM生成响应
func (a *LLMAdapter) Call(ctx context.Context, prompt string) (string, error) {
	if a.modelConfig == nil {
		return "", fmt.Errorf("model config is nil")
	}

	// 创建ModelService
	modelService := model.NewModelService(
		a.modelConfig.APIKey,
		a.modelConfig.BaseURL,
		nil, // Formatter可以为空
	)

	// 构建消息
	messages := []*schema.Message{
		{
			Role:    "user",
			Content: prompt,
		},
	}

	// 调用LLM
	params := model.ChatCompletionParams{
		ModelName:           a.modelConfig.Name,
		Messages:            messages,
		Temperature:         0.1, // 使用较低的temperature以获得更确定性的结果
		MaxCompletionTokens: 4000,
	}

	resp, err := modelService.ChatCompletion(ctx, params)
	if err != nil {
		g.Log().Errorf(ctx, "LLM call failed: %v", err)
		return "", err
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from LLM")
	}

	content := resp.Choices[0].Message.Content

	return content, nil
}

// VectorSearchAdapter 向量搜索适配器,将VectorStore适配为NL2SQL所需的向量搜索接口
type VectorSearchAdapter struct {
	vectorStore          vector_store.VectorStore
	knowledgeBaseID      string
	datasourceID         string
	embeddingModelConfig *model.EmbeddingModelConfig
}

// NewVectorSearchAdapter 创建向量搜索适配器
func NewVectorSearchAdapter(
	vectorStore vector_store.VectorStore,
	knowledgeBaseID string,
	datasourceID string,
	embeddingModelConfig *model.EmbeddingModelConfig,
) *VectorSearchAdapter {
	return &VectorSearchAdapter{
		vectorStore:          vectorStore,
		knowledgeBaseID:      knowledgeBaseID,
		datasourceID:         datasourceID,
		embeddingModelConfig: embeddingModelConfig,
	}
}

// VectorSearchResult 向量搜索结果
type VectorSearchResult struct {
	DocumentID string  // 对应NL2SQL中的entity_id
	ChunkID    string  // chunk_id
	Score      float64 // 相似度分数
}

// Search 执行向量搜索
func (a *VectorSearchAdapter) Search(ctx context.Context, query string, topK int) ([]VectorSearchResult, error) {
	// 构建检索配置，包含embedding模型配置
	retrieverConfig := &SimpleRetrieverConfig{
		topK:            topK,
		score:           0.3, // 默认分数阈值
		enableRewrite:   false,
		rewriteAttempts: 0,
		retrieveMode:    "simple",
	}

	// 如果有embedding模型配置，传递给retriever
	if a.embeddingModelConfig != nil {
		retrieverConfig.apiKey = a.embeddingModelConfig.APIKey
		retrieverConfig.baseURL = a.embeddingModelConfig.BaseURL
		retrieverConfig.embeddingModel = a.embeddingModelConfig.Name
	}

	// 执行向量搜索
	documents, err := a.vectorStore.VectorSearchOnlyNL2SQL(
		ctx,
		query,
		a.knowledgeBaseID,
		a.datasourceID,
		topK,
		0.3, // 分数阈值
	)
	if err != nil {
		g.Log().Errorf(ctx, "NL2SQL vector search failed: %v", err)
		return nil, err
	}

	// 转换为NL2SQL需要的格式
	var results []VectorSearchResult
	for _, doc := range documents {
		// 从NL2SQL表结构的metadata中提取entity_id
		entityID := ""
		if eid, ok := doc.MetaData[vector_store.NL2SQLFieldEntityId].(string); ok {
			entityID = eid
		}

		results = append(results, VectorSearchResult{
			DocumentID: entityID, // entity_id
			ChunkID:    doc.ID,   // chunk id
			Score:      float64(doc.Score),
		})
	}

	return results, nil
}

// SimpleRetrieverConfig 简单检索配置实现
type SimpleRetrieverConfig struct {
	topK            int
	score           float64
	enableRewrite   bool
	rewriteAttempts int
	retrieveMode    string
	apiKey          string
	baseURL         string
	embeddingModel  string
}

func (c *SimpleRetrieverConfig) GetTopK() int              { return c.topK }
func (c *SimpleRetrieverConfig) GetScore() float64         { return c.score }
func (c *SimpleRetrieverConfig) GetEnableRewrite() bool    { return c.enableRewrite }
func (c *SimpleRetrieverConfig) GetRewriteAttempts() int   { return c.rewriteAttempts }
func (c *SimpleRetrieverConfig) GetRetrieveMode() string   { return c.retrieveMode }
func (c *SimpleRetrieverConfig) GetAPIKey() string         { return c.apiKey }
func (c *SimpleRetrieverConfig) GetBaseURL() string        { return c.baseURL }
func (c *SimpleRetrieverConfig) GetEmbeddingModel() string { return c.embeddingModel }
