package knowledge_retrieval

import (
	"fmt"

	"github.com/Malowking/kbgo/core/agent_tools/framework"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// KnowledgeRetrievalToolAdapter 知识检索工具适配器
type KnowledgeRetrievalToolAdapter struct {
	tool *KnowledgeRetrievalTool
}

// NewKnowledgeRetrievalToolAdapter 创建知识检索工具适配器
func NewKnowledgeRetrievalToolAdapter() *KnowledgeRetrievalToolAdapter {
	return &KnowledgeRetrievalToolAdapter{
		tool: NewKnowledgeRetrievalTool(),
	}
}

// Execute 执行知识检索（框架适配）
func (t *KnowledgeRetrievalToolAdapter) Execute(
	ctx framework.ToolContext,
	config *KnowledgeRetrievalConfig,
	params map[string]interface{},
) (*schema.ToolResult, error) {
	// 提取查询参数
	query, ok := params["query"].(string)
	if !ok {
		query = ""
	}

	g.Log().Infof(ctx.Context, "[KnowledgeRetrievalTool] Starting retrieval, query: %s", query)

	// 执行知识检索
	result, err := t.tool.Execute(ctx.Context, config, query)
	if err != nil {
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("知识检索失败: %v", err), ""),
		), nil
	}

	// 构建返回内容
	var content string
	if len(result.Documents) > 0 {
		content = fmt.Sprintf("检索到 %d 个相关文档", len(result.Documents))
	} else {
		content = "未检索到相关文档"
	}

	// 创建 ToolResult
	toolResult := schema.NewToolResult(content)

	// 添加文档作为引用
	for i, doc := range result.Documents {
		citation := schema.NewCitation(
			"knowledge_base",
			fmt.Sprintf("文档 %d", i+1),
			"",
			doc.Content,
		)
		toolResult.WithCitation(citation)
	}

	// 添加元数据
	toolResult.WithMetadata("document_count", len(result.Documents))
	toolResult.WithMetadata("knowledge_id", config.KnowledgeID)

	g.Log().Infof(ctx.Context, "[KnowledgeRetrievalTool] Retrieval completed: %d documents", len(result.Documents))

	return toolResult, nil
}

// ParseConfig 从输入参数中解析配置并校验必要参数
func ParseConfig(input map[string]interface{}) (*KnowledgeRetrievalConfig, error) {
	config := &KnowledgeRetrievalConfig{
		EnableRewrite: true, // 默认启用查询重写
	}

	if knowledgeID, ok := input["knowledge_id"].(string); ok {
		config.KnowledgeID = knowledgeID
	}

	if topK, ok := input["top_k"].(float64); ok {
		config.TopK = int(topK)
	} else if topK, ok := input["top_k"].(int); ok {
		config.TopK = topK
	}

	if score, ok := input["score"].(float64); ok {
		config.Score = score
	}

	if retrieveMode, ok := input["retrieve_mode"].(string); ok {
		config.RetrieveMode = retrieveMode
	}

	if enableRewrite, ok := input["enable_rewrite"].(bool); ok {
		config.EnableRewrite = enableRewrite
	}

	if rewriteAttempts, ok := input["rewrite_attempts"].(float64); ok {
		config.RewriteAttempts = int(rewriteAttempts)
	} else if rewriteAttempts, ok := input["rewrite_attempts"].(int); ok {
		config.RewriteAttempts = rewriteAttempts
	}

	if rerankWeight, ok := input["rerank_weight"].(float64); ok {
		config.RerankWeight = &rerankWeight
	}

	if rerankModelID, ok := input["rerank_model_id"].(string); ok {
		config.RerankModelID = rerankModelID
	}

	query, _ := input["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("缺少必需参数 'query'")
	}

	return config, nil
}
