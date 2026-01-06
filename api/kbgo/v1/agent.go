package v1

import (
	"mime/multipart"

	"github.com/gogf/gf/v2/frame/g"
)

// ============ Agent预设管理接口 ============

// CreateAgentPresetReq 创建Agent预设请求
type CreateAgentPresetReq struct {
	g.Meta      `path:"/v1/agent/preset" method:"post" tags:"agent" summary:"创建Agent预设"`
	PresetName  string        `json:"preset_name" v:"required#预设名称不能为空"` // 预设名称
	Description string        `json:"description"`                       // 预设描述
	Config      AgentConfig   `json:"config" v:"required#配置不能为空"`        // Agent配置
	Tools       []*ToolConfig `json:"tools"`                             // 工具配置（独立存储）
	IsPublic    bool          `json:"is_public"`                         // 是否公开分享
	UserID      string        `json:"user_id" v:"required#用户ID不能为空"`     // 用户ID
}

// AgentConfig Agent配置结构（对应ChatReq的核心参数，不包含Tools）
type AgentConfig struct {
	ModelID          string   `json:"model_id" v:"required#模型ID不能为空"` // LLM模型UUID
	SystemPrompt     string   `json:"system_prompt"`                  // 系统提示词
	EmbeddingModelID string   `json:"embedding_model_id"`             // Embedding模型UUID（可选，如果不提供且启用检索器，则使用知识库绑定的模型）
	RerankModelID    string   `json:"rerank_model_id"`                // Rerank模型UUID
	KnowledgeId      string   `json:"knowledge_id"`                   // 知识库ID
	EnableRetriever  bool     `json:"enable_retriever"`               // 是否启用检索器
	TopK             int      `json:"top_k"`                          // 检索Top K（默认5）
	Score            float64  `json:"score"`                          // 相似度分数阈值（默认0.2）
	RetrieveMode     string   `json:"retrieve_mode"`                  // 检索模式: simple（普通检索）/rerank/rrf
	RerankWeight     *float64 `json:"rerank_weight"`                  // Rerank权重 (0-1范围，默认1.0)
	JsonFormat       bool     `json:"jsonformat"`                     // 是否需要JSON格式化输出
}

// CreateAgentPresetRes 创建Agent预设响应
type CreateAgentPresetRes struct {
	PresetID string `json:"preset_id"` // 预设ID
}

// UpdateAgentPresetReq 更新Agent预设请求
type UpdateAgentPresetReq struct {
	g.Meta      `path:"/v1/agent/preset/:preset_id" method:"put" tags:"agent" summary:"更新Agent预设"`
	PresetID    string        `json:"preset_id" v:"required#预设ID不能为空"` // 预设ID
	PresetName  string        `json:"preset_name"`                     // 预设名称
	Description string        `json:"description"`                     // 预设描述
	Config      AgentConfig   `json:"config"`                          // Agent配置
	Tools       []*ToolConfig `json:"tools"`                           // 工具配置（独立存储）
	IsPublic    bool          `json:"is_public"`                       // 是否公开分享
	UserID      string        `json:"user_id" v:"required#用户ID不能为空"`   // 用户ID（用于权限验证）
}

// UpdateAgentPresetRes 更新Agent预设响应
type UpdateAgentPresetRes struct {
	Success bool `json:"success"` // 是否成功
}

// GetAgentPresetReq 获取Agent预设请求
type GetAgentPresetReq struct {
	g.Meta   `path:"/v1/agent/preset/:preset_id" method:"get" tags:"agent" summary:"获取Agent预设详情"`
	PresetID string `json:"preset_id" v:"required#预设ID不能为空"` // 预设ID
}

// GetAgentPresetRes 获取Agent预设响应
type GetAgentPresetRes struct {
	PresetID    string        `json:"preset_id"`   // 预设ID
	UserID      string        `json:"user_id"`     // 用户ID
	PresetName  string        `json:"preset_name"` // 预设名称
	Description string        `json:"description"` // 预设描述
	Config      AgentConfig   `json:"config"`      // Agent配置
	Tools       []*ToolConfig `json:"tools"`       // 工具配置（独立存储）
	IsPublic    bool          `json:"is_public"`   // 是否公开
	CreateTime  string        `json:"create_time"` // 创建时间
	UpdateTime  string        `json:"update_time"` // 更新时间
}

// ListAgentPresetsReq 获取Agent预设列表请求
type ListAgentPresetsReq struct {
	g.Meta   `path:"/v1/agent/presets" method:"get" tags:"agent" summary:"获取Agent预设列表"`
	UserID   string `json:"user_id"`                       // 用户ID（查询该用户的预设）
	IsPublic *bool  `json:"is_public"`                     // 是否只查询公开的预设（null=全部，true=公开，false=私有）
	Page     int    `json:"page" v:"min:1#页码必须大于0"`        // 页码
	PageSize int    `json:"page_size" v:"min:1#每页数量必须大于0"` // 每页数量
}

// ListAgentPresetsRes 获取Agent预设列表响应
type ListAgentPresetsRes struct {
	List  []*AgentPresetItem `json:"list"`  // 预设列表
	Total int64              `json:"total"` // 总数
	Page  int                `json:"page"`  // 当前页码
}

// AgentPresetItem Agent预设列表项
type AgentPresetItem struct {
	PresetID    string `json:"preset_id"`   // 预设ID
	PresetName  string `json:"preset_name"` // 预设名称
	Description string `json:"description"` // 预设描述
	IsPublic    bool   `json:"is_public"`   // 是否公开
	CreateTime  string `json:"create_time"` // 创建时间
	UpdateTime  string `json:"update_time"` // 更新时间
}

// DeleteAgentPresetReq 删除Agent预设请求
type DeleteAgentPresetReq struct {
	g.Meta   `path:"/v1/agent/preset/:preset_id" method:"delete" tags:"agent" summary:"删除Agent预设"`
	PresetID string `json:"preset_id" v:"required#预设ID不能为空"` // 预设ID
	UserID   string `json:"user_id" v:"required#用户ID不能为空"`   // 用户ID（用于权限验证）
}

// DeleteAgentPresetRes 删除Agent预设响应
type DeleteAgentPresetRes struct {
	Success bool `json:"success"` // 是否成功
}

// ============ Agent调用接口 ============

// AgentChatReq Agent对话请求
type AgentChatReq struct {
	g.Meta   `path:"/v1/agent/chat" method:"post" tags:"agent" summary:"使用Agent预设进行对话"`
	PresetID string                  `json:"preset_id" v:"required#预设ID不能为空"` // Agent预设ID
	ConvID   string                  `json:"conv_id"`                         // 会话ID（可选，首次为空会创建新会话）
	UserID   string                  `json:"user_id" v:"required#用户ID不能为空"`   // 用户ID
	Question string                  `json:"question" v:"required#问题不能为空"`    // 用户问题
	Stream   bool                    `json:"stream"`                          // 是否流式返回
	Files    []*multipart.FileHeader `json:"files" type:"file"`               // 多模态文件（图片、音频、视频等）
}

// AgentChatRes Agent对话响应
type AgentChatRes struct {
	ConvID           string       `json:"conv_id"`                     // 会话ID（新会话会返回创建的conv_id）
	Answer           string       `json:"answer"`                      // 回答内容
	ReasoningContent string       `json:"reasoning_content,omitempty"` // 思考内容（用于思考模型）
	References       []*AgentDoc  `json:"references,omitempty"`        // 引用文档
	MCPResults       []*MCPResult `json:"mcp_results,omitempty"`       // MCP调用结果
}

// AgentDoc 引用文档结构
type AgentDoc struct {
	DocumentID string  `json:"document_id"` // 文档ID
	ChunkID    string  `json:"chunk_id"`    // 片段ID
	Content    string  `json:"content"`     // 内容
	Score      float64 `json:"score"`       // 相似度分数
}
