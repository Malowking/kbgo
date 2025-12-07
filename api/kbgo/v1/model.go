package v1

import (
	"github.com/Malowking/kbgo/core/model"
	"github.com/gogf/gf/v2/frame/g"
)

// ReloadModelsReq 重新加载模型配置请求
type ReloadModelsReq struct {
	g.Meta `path:"/v1/model/reload" method:"post" tags:"model" summary:"Reload models from database"`
}

// ReloadModelsRes 重新加载模型配置响应
type ReloadModelsRes struct {
	g.Meta  `mime:"application/json"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	Count   int    `json:"count"` // 加载的模型数量
}

// ListModelsReq 列出模型请求
type ListModelsReq struct {
	g.Meta    `path:"/v1/model/list" method:"get" tags:"model" summary:"List all models"`
	ModelType string `json:"model_type"` // 可选，按类型过滤：llm, embedding, reranker, multimodal, image, video, audio
}

// ListModelsRes 列出模型响应
type ListModelsRes struct {
	g.Meta `mime:"application/json"`
	Models []*model.ModelConfig `json:"models"`
	Count  int                  `json:"count"`
}

// GetModelReq 获取模型详情请求
type GetModelReq struct {
	g.Meta  `path:"/v1/model/:model_id" method:"get" tags:"model" summary:"Get model details"`
	ModelID string `json:"model_id" v:"required"` // 模型UUID
}

// GetModelRes 获取模型详情响应
type GetModelRes struct {
	g.Meta `mime:"application/json"`
	Model  *model.ModelConfig `json:"model"`
}

// ChatCompletionReq OpenAI 风格聊天请求
type ChatCompletionReq struct {
	g.Meta           `path:"/v1/model/chat" method:"post" tags:"model" summary:"Chat with model (OpenAI style)"`
	ModelID          string                  `json:"model_id" v:"required"` // 模型UUID
	Messages         []ChatCompletionMessage `json:"messages" v:"required"` // 消息列表
	MaxTokens        int                     `json:"max_tokens"`            // 最大生成token数（可选，默认4096）
	Temperature      float32                 `json:"temperature"`           // 温度（可选，默认0.7）
	TopP             float32                 `json:"top_p"`                 // 核采样（可选，默认0.9）
	FrequencyPenalty float32                 `json:"frequency_penalty"`     // 频率惩罚（可选，默认0.0）
	PresencePenalty  float32                 `json:"presence_penalty"`      // 存在惩罚（可选，默认0.0）
	Stop             []string                `json:"stop"`                  // 停止词（可选）
	Stream           bool                    `json:"stream"`                // 是否流式返回（可选，默认false）
	Tools            []ChatCompletionTool    `json:"tools"`                 // 工具列表（可选）
	ToolChoice       interface{}             `json:"tool_choice"`           // 工具选择策略（可选）
}

// ChatCompletionMessage 聊天消息
type ChatCompletionMessage struct {
	Role       string                   `json:"role"`         // system, user, assistant, tool
	Content    string                   `json:"content"`      // 消息内容
	Name       string                   `json:"name"`         // 可选，消息发送者名称
	ToolCallID string                   `json:"tool_call_id"` // 工具调用ID（role=tool时必填）
	ToolCalls  []ChatCompletionToolCall `json:"tool_calls"`   // 工具调用列表
}

// ChatCompletionToolCall 工具调用
type ChatCompletionToolCall struct {
	ID       string                     `json:"id"`
	Type     string                     `json:"type"` // function
	Function ChatCompletionToolCallFunc `json:"function"`
}

// ChatCompletionToolCallFunc 工具调用函数
type ChatCompletionToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON字符串
}

// ChatCompletionTool 工具定义
type ChatCompletionTool struct {
	Type     string                     `json:"type"` // function
	Function ChatCompletionToolFunction `json:"function"`
}

// ChatCompletionToolFunction 工具函数定义
type ChatCompletionToolFunction struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// ChatCompletionRes OpenAI 风格聊天响应
type ChatCompletionRes struct {
	g.Meta  `mime:"application/json"`
	ID      string                 `json:"id"`
	Object  string                 `json:"object"` // chat.completion
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   ChatCompletionUsage    `json:"usage"`
}

// ChatCompletionChoice 响应选项
type ChatCompletionChoice struct {
	Index        int                   `json:"index"`
	Message      ChatCompletionMessage `json:"message"`
	FinishReason string                `json:"finish_reason"` // stop, length, tool_calls
}

// ChatCompletionUsage token使用情况
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// EmbeddingReq 向量化请求
type EmbeddingReq struct {
	g.Meta  `path:"/v1/model/embeddings" method:"post" tags:"model" summary:"Create embeddings"`
	ModelID string   `json:"model_id" v:"required"` // 模型UUID
	Input   []string `json:"input" v:"required"`    // 输入文本列表
}

// EmbeddingRes 向量化响应
type EmbeddingRes struct {
	g.Meta `mime:"application/json"`
	Object string          `json:"object"` // list
	Data   []EmbeddingData `json:"data"`
	Model  string          `json:"model"`
	Usage  EmbeddingUsage  `json:"usage"`
}

// EmbeddingData 向量数据
type EmbeddingData struct {
	Index     int       `json:"index"`
	Embedding []float32 `json:"embedding"`
}

// EmbeddingUsage 向量化使用情况
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// RegisterModelReq 注册模型请求
type RegisterModelReq struct {
	g.Meta              `path:"/v1/model/register" method:"post" tags:"model" summary:"Register a new model"`
	ModelName           string                 `json:"model_name" v:"required"`                                                        // 模型名称
	ModelType           string                 `json:"model_type" v:"required|in:llm,embedding,reranker,multimodal,image,video,audio"` // 模型类型
	Provider            string                 `json:"provider"`                                                                       // 提供商（openai, ollama等）（可选）
	BaseURL             string                 `json:"base_url"`                                                                       // API基础URL（可选）
	APIKey              string                 `json:"api_key"`                                                                        // API密钥（可选）
	MaxCompletionTokens int                    `json:"max_completion_tokens"`                                                          // 最大输出token数（可选）
	Dimension           int                    `json:"dimension"`                                                                      // 向量维度（embedding模型专用）
	Config              map[string]interface{} `json:"config"`                                                                         // 其他配置（可选）
	Enabled             bool                   `json:"enabled"`                                                                        // 是否启用（默认true）
}

// RegisterModelRes 注册模型响应
type RegisterModelRes struct {
	g.Meta  `mime:"application/json"`
	Success bool   `json:"success"`
	Message string `json:"message"`
	ModelID string `json:"model_id"`
}

// UpdateModelReq 更新模型请求
type UpdateModelReq struct {
	g.Meta    `path:"/v1/model/:model_id" method:"put" tags:"model" summary:"Update model configuration"`
	ModelID   string  `json:"model_id" v:"required"` // 模型ID（路径参数，必传）
	ModelName *string `json:"model_name"`            // 模型名称（可选）
	ModelType *string `json:"model_type"`            // 模型类型（可选）
	Provider  *string `json:"provider"`              // 提供商（可选）
	Version   *string `json:"version"`               // 版本（可选）
	BaseURL   *string `json:"base_url"`              // API基础URL（可选）
	APIKey    *string `json:"api_key"`               // API密钥（可选）
	Enabled   *bool   `json:"enabled"`               // 是否启用（可选）
	Extra     *string `json:"extra"`                 // 额外配置参数，JSON字符串（可选）
}

// UpdateModelRes 更新模型响应
type UpdateModelRes struct {
	g.Meta  `mime:"application/json"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// DeleteModelReq 删除模型请求
type DeleteModelReq struct {
	g.Meta  `path:"/v1/model/:model_id" method:"delete" tags:"model" summary:"Delete a model"`
	ModelID string `json:"model_id" v:"required"` // 模型ID
}

// DeleteModelRes 删除模型响应
type DeleteModelRes struct {
	g.Meta  `mime:"application/json"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}
