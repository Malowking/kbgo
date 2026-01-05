package knowledge_retrieval

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/logic/retriever"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// KnowledgeRetrievalTool 知识检索工具
type KnowledgeRetrievalTool struct{}

// NewKnowledgeRetrievalTool 创建知识检索工具
func NewKnowledgeRetrievalTool() *KnowledgeRetrievalTool {
	return &KnowledgeRetrievalTool{}
}

// KnowledgeRetrievalConfig 知识检索工具配置
type KnowledgeRetrievalConfig struct {
	KnowledgeID      string   `json:"knowledge_id"`       // 知识库ID（必填）
	TopK             int      `json:"top_k"`              // 返回文档数量（默认5）
	Score            float64  `json:"score"`              // 相似度阈值（默认0.2）
	RetrieveMode     string   `json:"retrieve_mode"`      // 检索模式: simple/rerank/rrf（默认rrf）
	EnableRewrite    bool     `json:"enable_rewrite"`     // 是否启用查询重写（默认true）
	RewriteAttempts  int      `json:"rewrite_attempts"`   // 查询重写次数（默认3）
	RerankWeight     *float64 `json:"rerank_weight"`      // Rerank权重0-1（默认1.0）
	EmbeddingModelID string   `json:"embedding_model_id"` // Embedding模型ID（可选，默认使用知识库绑定模型）
	RerankModelID    string   `json:"rerank_model_id"`    // Rerank模型ID（可选）
}

// KnowledgeRetrievalResult 知识检索结果
type KnowledgeRetrievalResult struct {
	Documents []*schema.Document
	Error     string
}

// Execute 执行知识检索
func (t *KnowledgeRetrievalTool) Execute(ctx context.Context, config *KnowledgeRetrievalConfig, question string) (*KnowledgeRetrievalResult, error) {
	result := &KnowledgeRetrievalResult{
		Documents: make([]*schema.Document, 0),
	}

	// 验证必填参数
	if config.KnowledgeID == "" {
		g.Log().Warning(ctx, "Knowledge retrieval: knowledge_id is empty, skipping retrieval")
		return result, nil
	}

	// 设置默认值
	topK := config.TopK
	if topK == 0 {
		topK = 5
	}

	score := config.Score
	if score == 0 {
		score = 0.2
	}

	retrieveMode := config.RetrieveMode
	if retrieveMode == "" {
		retrieveMode = "rrf"
	}

	rewriteAttempts := config.RewriteAttempts
	if rewriteAttempts == 0 && config.EnableRewrite {
		rewriteAttempts = 3
	}

	g.Log().Infof(ctx, "Executing knowledge retrieval: KnowledgeID=%s, TopK=%d, Score=%f, Mode=%s, EnableRewrite=%v",
		config.KnowledgeID, topK, score, retrieveMode, config.EnableRewrite)

	// 调用检索逻辑
	retrieverRes, err := retriever.ProcessRetrieval(ctx, &v1.RetrieverReq{
		Question:         question,
		EmbeddingModelID: config.EmbeddingModelID,
		RerankModelID:    config.RerankModelID,
		TopK:             topK,
		Score:            score,
		KnowledgeId:      config.KnowledgeID,
		EnableRewrite:    config.EnableRewrite,
		RewriteAttempts:  rewriteAttempts,
		RetrieveMode:     retrieveMode,
		RerankWeight:     config.RerankWeight,
	})

	if err != nil {
		g.Log().Errorf(ctx, "Knowledge retrieval failed: %v", err)
		result.Error = err.Error()
		return result, err
	}

	result.Documents = retrieverRes.Document
	g.Log().Infof(ctx, "Knowledge retrieval completed: Retrieved %d documents", len(retrieverRes.Document))

	return result, nil
}

// ParseConfig 从map[string]interface{}解析配置
func ParseConfig(configMap map[string]interface{}) *KnowledgeRetrievalConfig {
	config := &KnowledgeRetrievalConfig{
		EnableRewrite: true, // 默认启用查询重写
	}

	if knowledgeID, ok := configMap["knowledge_id"].(string); ok {
		config.KnowledgeID = knowledgeID
	}

	if topK, ok := configMap["top_k"].(float64); ok {
		config.TopK = int(topK)
	} else if topK, ok := configMap["top_k"].(int); ok {
		config.TopK = topK
	}

	if score, ok := configMap["score"].(float64); ok {
		config.Score = score
	}

	if retrieveMode, ok := configMap["retrieve_mode"].(string); ok {
		config.RetrieveMode = retrieveMode
	}

	if enableRewrite, ok := configMap["enable_rewrite"].(bool); ok {
		config.EnableRewrite = enableRewrite
	}

	if rewriteAttempts, ok := configMap["rewrite_attempts"].(float64); ok {
		config.RewriteAttempts = int(rewriteAttempts)
	} else if rewriteAttempts, ok := configMap["rewrite_attempts"].(int); ok {
		config.RewriteAttempts = rewriteAttempts
	}

	if rerankWeight, ok := configMap["rerank_weight"].(float64); ok {
		config.RerankWeight = &rerankWeight
	}

	if embeddingModelID, ok := configMap["embedding_model_id"].(string); ok {
		config.EmbeddingModelID = embeddingModelID
	}

	if rerankModelID, ok := configMap["rerank_model_id"].(string); ok {
		config.RerankModelID = rerankModelID
	}

	return config
}
