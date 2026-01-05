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
	modelConfig *model.ModelConfig
}

// NewLLMAdapter 创建LLM适配器
func NewLLMAdapter(modelConfig *model.ModelConfig) *LLMAdapter {
	return &LLMAdapter{
		modelConfig: modelConfig,
	}
}

// Call 调用LLM生成响应
func (a *LLMAdapter) Call(ctx context.Context, prompt string) (string, error) {
	if a.modelConfig == nil {
		return "", fmt.Errorf("model config is nil")
	}

	g.Log().Debugf(ctx, "LLMAdapter calling model: %s", a.modelConfig.ModelID)

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
	g.Log().Debugf(ctx, "LLM response length: %d chars", len(content))

	return content, nil
}

// VectorSearchAdapter 向量搜索适配器,将VectorStore适配为NL2SQL所需的向量搜索接口
type VectorSearchAdapter struct {
	vectorStore          vector_store.VectorStore
	knowledgeBaseID      string
	embeddingModelConfig *model.ModelConfig
}

// NewVectorSearchAdapter 创建向量搜索适配器
func NewVectorSearchAdapter(
	vectorStore vector_store.VectorStore,
	knowledgeBaseID string,
	embeddingModelConfig *model.ModelConfig,
) *VectorSearchAdapter {
	return &VectorSearchAdapter{
		vectorStore:          vectorStore,
		knowledgeBaseID:      knowledgeBaseID,
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
	g.Log().Debugf(ctx, "VectorSearchAdapter searching: query=%s, topK=%d, kb=%s",
		query, topK, a.knowledgeBaseID)

	// 构建检索配置
	retrieverConfig := &SimpleRetrieverConfig{
		topK:            topK,
		score:           0.3, // 默认分数阈值
		enableRewrite:   false,
		rewriteAttempts: 0,
		retrieveMode:    "simple",
	}

	// 执行向量搜索
	documents, err := a.vectorStore.VectorSearchOnly(
		ctx,
		retrieverConfig,
		query,
		a.knowledgeBaseID,
		topK,
		0.3, // 分数阈值
	)
	if err != nil {
		g.Log().Errorf(ctx, "Vector search failed: %v", err)
		return nil, err
	}

	// 转换为NL2SQL需要的格式
	var results []VectorSearchResult
	for _, doc := range documents {
		results = append(results, VectorSearchResult{
			DocumentID: doc.MetaData["document_id"].(string),
			ChunkID:    doc.MetaData["chunk_id"].(string),
			Score:      float64(doc.Score),
		})
	}

	g.Log().Debugf(ctx, "Vector search found %d results", len(results))
	return results, nil
}

// SimpleRetrieverConfig 简单检索配置实现
type SimpleRetrieverConfig struct {
	topK            int
	score           float64
	enableRewrite   bool
	rewriteAttempts int
	retrieveMode    string
}

func (c *SimpleRetrieverConfig) GetTopK() int            { return c.topK }
func (c *SimpleRetrieverConfig) GetScore() float64       { return c.score }
func (c *SimpleRetrieverConfig) GetEnableRewrite() bool  { return c.enableRewrite }
func (c *SimpleRetrieverConfig) GetRewriteAttempts() int { return c.rewriteAttempts }
func (c *SimpleRetrieverConfig) GetRetrieveMode() string { return c.retrieveMode }
