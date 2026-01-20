package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// ConversationListReq 会话列表请求
type ConversationListReq struct {
	g.Meta           `path:"/v1/conversations" method:"get" tags:"conversation"`
	KnowledgeID      string `json:"knowledge_id" v:""`       // 知识库ID（可选，用于筛选）
	ConversationType string `json:"conversation_type" v:""`  // 会话类型（可选，用于筛选）：text/agent
	AgentPresetID    string `json:"agent_preset_id" v:""`    // Agent预设ID（可选，用于筛选Agent对话）
	Page             int    `json:"page" d:"1"`              // 页码，默认1
	PageSize         int    `json:"page_size" d:"20"`        // 每页数量，默认20
	Status           string `json:"status" v:""`             // 状态筛选：active/archived
	SortBy           string `json:"sort_by" d:"update_time"` // 排序字段：create_time/update_time/message_count
	Order            string `json:"order" d:"desc"`          // 排序方向：asc/desc
}

// ConversationListRes 会话列表响应
type ConversationListRes struct {
	g.Meta        `mime:"application/json"`
	Conversations []*ConversationItem `json:"conversations"`
	Total         int64               `json:"total"`
	Page          int                 `json:"page"`
	PageSize      int                 `json:"page_size"`
}

// ConversationItem 会话列表项
type ConversationItem struct {
	ConvID           string         `json:"conv_id"`
	Title            string         `json:"title"`
	ModelID          string         `json:"model_id"`
	ConversationType string         `json:"conversation_type"`
	Status           string         `json:"status"`
	MessageCount     int            `json:"message_count"`     // 消息数量
	LastMessage      string         `json:"last_message"`      // 最后一条消息摘要
	LastMessageTime  string         `json:"last_message_time"` // 最后消息时间
	CreateTime       string         `json:"create_time"`
	UpdateTime       string         `json:"update_time"`
	AgentPresetID    string         `json:"agent_preset_id"`    // Agent预设ID
	Tags             []string       `json:"tags,omitempty"`     // 标签
	Metadata         map[string]any `json:"metadata,omitempty"` // 元数据
}

// ConversationDetailReq 会话详情请求
type ConversationDetailReq struct {
	g.Meta `path:"/v1/conversations/:conv_id" method:"get" tags:"conversation"`
	ConvID string `json:"conv_id" v:"required"`
}

// ConversationDetailRes 会话详情响应
type ConversationDetailRes struct {
	g.Meta           `mime:"application/json"`
	ConvID           string         `json:"conv_id"`
	UserID           string         `json:"user_id"`
	Title            string         `json:"title"`
	ModelID          string         `json:"model_id"`
	ConversationType string         `json:"conversation_type"`
	Status           string         `json:"status"`
	MessageCount     int            `json:"message_count"`
	Messages         []*MessageItem `json:"messages"` // 消息列表
	CreateTime       string         `json:"create_time"`
	UpdateTime       string         `json:"update_time"`
	Tags             []string       `json:"tags,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// MessageItem 消息项
type MessageItem struct {
	MsgID            string         `json:"msg_id"`                      // 消息ID
	Role             string         `json:"role"`                        // 角色：user/assistant/system/tool
	Content          *string        `json:"content"`                     // 文本内容（可为null）
	ToolCalls        []ToolCall     `json:"tool_calls,omitempty"`        // 工具调用列表
	ToolCallID       string         `json:"tool_call_id,omitempty"`      // 工具调用ID（tool角色使用）
	ReasoningContent string         `json:"reasoning_content,omitempty"` // 思考内容
	CreateTime       string         `json:"create_time"`                 // 创建时间
	Extra            map[string]any `json:"extra,omitempty"`             // 扩展字段
}

// ToolCall 工具调用
type ToolCall struct {
	ID       string       `json:"id"`       // 工具调用ID
	Type     string       `json:"type"`     // 类型，通常为 "function"
	Function FunctionCall `json:"function"` // 函数调用信息
}

// FunctionCall 函数调用
type FunctionCall struct {
	Name      string `json:"name"`      // 函数名称
	Arguments string `json:"arguments"` // 函数参数（JSON字符串）
}

// ConversationDeleteReq 删除会话请求
type ConversationDeleteReq struct {
	g.Meta `path:"/v1/conversations/:conv_id" method:"delete" tags:"conversation"`
	ConvID string `json:"conv_id" v:"required"`
}

// ConversationDeleteRes 删除会话响应
type ConversationDeleteRes struct {
	g.Meta  `mime:"application/json"`
	Message string `json:"message"`
}

// ConversationUpdateReq 更新会话请求
type ConversationUpdateReq struct {
	g.Meta   `path:"/v1/conversations/:conv_id" method:"put" tags:"conversation"`
	ConvID   string         `json:"conv_id" v:"required"`
	Title    string         `json:"title" v:""`                    // 会话标题
	Status   string         `json:"status" v:"in:active,archived"` // 状态：active/archived
	Tags     []string       `json:"tags"`                          // 标签
	Metadata map[string]any `json:"metadata"`                      // 元数据
}

// ConversationUpdateRes 更新会话响应
type ConversationUpdateRes struct {
	g.Meta  `mime:"application/json"`
	Message string `json:"message"`
}

// ConversationSummaryReq 生成会话摘要请求
type ConversationSummaryReq struct {
	g.Meta  `path:"/v1/conversations/:conv_id/summary" method:"post" tags:"conversation"`
	ConvID  string `json:"conv_id" v:"required"`
	ModelID string `json:"model_id" v:"required"`                      // 用于生成摘要的模型ID
	Length  string `json:"length" d:"medium" v:"in:short,medium,long"` // 摘要长度
}

// ConversationSummaryRes 生成会话摘要响应
type ConversationSummaryRes struct {
	g.Meta  `mime:"application/json"`
	Summary string `json:"summary"`
}

// ConversationExportReq 导出会话请求
type ConversationExportReq struct {
	g.Meta `path:"/v1/conversations/:conv_id/export" method:"post" tags:"conversation"`
	ConvID string `json:"conv_id" v:"required"`
	Format string `json:"format" d:"json" v:"in:json,markdown,txt"` // 导出格式
}

// ConversationExportRes 导出会话响应
type ConversationExportRes struct {
	g.Meta      `mime:"application/json"`
	Content     string `json:"content"`                // 导出的内容
	DownloadURL string `json:"download_url,omitempty"` // 下载链接（如果是文件）
	Filename    string `json:"filename"`
}

// ConversationBatchDeleteReq 批量删除会话请求
type ConversationBatchDeleteReq struct {
	g.Meta  `path:"/v1/conversations/batch/delete" method:"post" tags:"conversation"`
	ConvIDs []string `json:"conv_ids" v:"required"` // 会话ID列表
}

// ConversationBatchDeleteRes 批量删除会话响应
type ConversationBatchDeleteRes struct {
	g.Meta       `mime:"application/json"`
	DeletedCount int      `json:"deleted_count"`
	FailedConvs  []string `json:"failed_convs,omitempty"` // 删除失败的会话ID
	Message      string   `json:"message"`
}

// CreateAgentConversationReq 创建Agent对话请求
type CreateAgentConversationReq struct {
	g.Meta   `path:"/v1/conversations/agent" method:"post" tags:"conversation" summary:"创建Agent对话"`
	ConvID   string `json:"conv_id" v:"required#会话ID不能为空"`   // 会话ID（由前端生成）
	PresetID string `json:"preset_id" v:"required#预设ID不能为空"` // Agent预设ID
	UserID   string `json:"user_id" v:"required#用户ID不能为空"`   // 用户ID
	Title    string `json:"title"`                           // 对话标题（可选，默认使用Agent名称）
}

// CreateAgentConversationRes 创建Agent对话响应
type CreateAgentConversationRes struct {
	g.Meta `mime:"application/json"`
	ConvID string `json:"conv_id"` // 会话ID
}
