package v1

import (
	"mime/multipart"

	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ToolConfig 工具配置
type ToolConfig struct {
	Type    string                 `json:"type"`    // "local_tools" or "mcp"
	Enabled bool                   `json:"enabled"` // 是否启用该类型的工具
	Config  map[string]interface{} `json:"config"`  // 工具配置参数
}

type ChatReq struct {
	g.Meta           `path:"/v1/chat" method:"post" tags:"retriever" mime:"multipart/form-data"`
	ConvID           string                  `json:"conv_id" v:"required"` // 会话id
	Question         string                  `json:"question" v:"required"`
	ModelID          string                  `json:"model_id" v:"required"` // LLM模型UUID（必填）
	SystemPrompt     string                  `json:"system_prompt"`         // 系统提示词（可选）
	EmbeddingModelID string                  `json:"embedding_model_id"`    // Embedding模型UUID（可选，如果不提供且启用检索器，则使用知识库绑定的模型）
	RerankModelID    string                  `json:"rerank_model_id"`       // Rerank模型UUID（可选，仅在使用rerank或rrf检索模式时需要）
	KnowledgeId      string                  `json:"knowledge_id"`
	EnableRetriever  bool                    `json:"enable_retriever"`  // Whether to enable knowledge base retrieval
	TopK             int                     `json:"top_k"`             // 默认为5
	Score            float64                 `json:"score"`             // 默认为0.2 （默认是rrf检索模式，相似度分数不重要）
	RetrieveMode     string                  `json:"retrieve_mode"`     // 检索模式: simple（普通检索）/rerank/rrf (默认rerank)
	RerankWeight     *float64                `json:"rerank_weight"`     // Rerank权重 (0-1范围，默认1.0)，1.0为纯rerank，0.0为纯BM25，中间值为混合
	Stream           bool                    `json:"stream"`            // 是否流式返回
	JsonFormat       bool                    `json:"jsonformat"`        // 是否需要JSON格式化输出
	Files            []*multipart.FileHeader `json:"files" type:"file"` // 上传的多模态文件（图片、音频、视频）

	// 新的统一工具配置
	Tools []*ToolConfig `json:"tools"` // 统一的工具配置
}

type ChatRes struct {
	g.Meta           `mime:"application/json"`
	Answer           string             `json:"answer"`
	ReasoningContent string             `json:"reasoning_content,omitempty"` // 思考内容（用于思考模型）
	References       []*schema.Document `json:"references"`
	MCPResults       []*MCPResult       `json:"mcp_results,omitempty"`
	NL2SQLResult     *NL2SQLChatResult  `json:"nl2sql_result,omitempty"` // NL2SQL查询结果（如果启用）
}

// NL2SQLChatResult NL2SQL在Chat中的查询结果
type NL2SQLChatResult struct {
	QueryLogID      string                   `json:"query_log_id"`       // 查询日志ID
	SQL             string                   `json:"sql"`                // 生成的SQL
	Columns         []string                 `json:"columns"`            // 结果列名
	Data            []map[string]interface{} `json:"data"`               // 结果数据
	RowCount        int                      `json:"row_count"`          // 返回的行数
	TotalRowCount   int                      `json:"total_row_count"`    // 完整行数（如果被截断）
	Explanation     string                   `json:"explanation"`        // SQL解释
	Error           string                   `json:"error,omitempty"`    // 错误信息（如果有）
	IntentType      string                   `json:"intent_type"`        // 意图类型
	NeedLLMAnalysis bool                     `json:"need_llm_analysis"`  // 是否需要LLM分析
	AnalysisFocus   []string                 `json:"analysis_focus"`     // 分析重点
	DataTruncated   bool                     `json:"data_truncated"`     // 数据是否被截断
	FileURL         string                   `json:"file_url,omitempty"` // TODO: 大结果集文件下载URL
}

type MCPResult struct {
	ServiceName string `json:"service_name"`
	ToolName    string `json:"tool_name"`
	Content     string `json:"content"`
}

// ChatStreamReq 流式输出请求
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
