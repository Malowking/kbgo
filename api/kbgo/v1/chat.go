package v1

import (
	"mime/multipart"

	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

type ChatReq struct {
	g.Meta           `path:"/v1/chat" method:"post" tags:"retriever" mime:"multipart/form-data"`
	ConvID           string                  `json:"conv_id" v:"required"` // 会话id
	Question         string                  `json:"question" v:"required"`
	ModelID          string                  `json:"model_id" v:"required"` // LLM模型UUID（必填）
	EmbeddingModelID string                  `json:"embedding_model_id"`    // Embedding模型UUID（可选，启用检索器时需要）
	RerankModelID    string                  `json:"rerank_model_id"`       // Rerank模型UUID（可选，仅在使用rerank或rrf检索模式时需要）
	KnowledgeId      string                  `json:"knowledge_id"`
	EnableRetriever  bool                    `json:"enable_retriever"`  // Whether to enable knowledge base retrieval
	TopK             int                     `json:"top_k"`             // 默认为5
	Score            float64                 `json:"score"`             // 默认为0.2 （默认是rrf检索模式，相似度分数不重要）
	UseMCP           bool                    `json:"use_mcp"`           // 是否使用MCP
	MCPServiceTools  map[string][]string     `json:"mcp_service_tools"` // 按服务指定允许调用的MCP工具列表
	Stream           bool                    `json:"stream"`            // 是否流式返回
	Files            []*multipart.FileHeader `json:"files" type:"file"` // 上传的多模态文件（图片、音频、视频）
}

type ChatRes struct {
	g.Meta     `mime:"application/json"`
	Answer     string             `json:"answer"`
	References []*schema.Document `json:"references"`
	MCPResults []*MCPResult       `json:"mcp_results,omitempty"`
}

type MCPResult struct {
	ServiceName string `json:"service_name"`
	ToolName    string `json:"tool_name"`
	Content     string `json:"content"`
}

// ChatStreamReq 流式输出请求 (保留兼容性)
type ChatStreamReq struct {
	g.Meta      `path:"/v1/chat/stream" method:"post" tags:"retriever"`
	ConvID      string  `json:"conv_id" v:"required"` // Session ID
	Question    string  `json:"question" v:"required"`
	KnowledgeId string  `json:"knowledge_id"`
	TopK        int     `json:"top_k"` // Default is 5
	Score       float64 `json:"score"` // Default is 0.2 (similarity score is not important in RRF retrieval mode)
}

// ChatStreamRes Streaming output response
type ChatStreamRes struct {
	g.Meta `mime:"text/event-stream"`
	// Streaming output does not need to return specific content, content is returned via HTTP response stream
}
