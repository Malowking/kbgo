package chat

import (
	openaiSDK "github.com/sashabaranov/go-openai"
)

// ModelParams 封装大模型的推理参数
type ModelParams struct {
	// Temperature 控制输出的随机性，0.0-2.0，越小越确定性
	Temperature *float32 `json:"temperature,omitempty" yaml:"temperature,omitempty"`

	// TopP 核采样参数，0.0-1.0，控制生成的多样性
	TopP *float32 `json:"topP,omitempty" yaml:"topP,omitempty"`

	// MaxCompletionTokens 最大生成token数量
	MaxCompletionTokens *int `json:"maxCompletionTokens,omitempty" yaml:"maxCompletionTokens,omitempty"`

	// FrequencyPenalty 频率惩罚，减少重复词汇，-2.0到2.0
	FrequencyPenalty *float32 `json:"frequencyPenalty,omitempty" yaml:"frequencyPenalty,omitempty"`

	// PresencePenalty 存在惩罚，鼓励新话题，-2.0到2.0
	PresencePenalty *float32 `json:"presencePenalty,omitempty" yaml:"presencePenalty,omitempty"`

	// N 生成多少个回复选项，默认为1
	N *int `json:"n,omitempty" yaml:"n,omitempty"`

	// Stop 停止词列表
	Stop []string `json:"stop,omitempty" yaml:"stop,omitempty"`

	// Functions 函数调用配置
	Functions []Function `json:"functions,omitempty" yaml:"functions,omitempty"`

	// Tools 工具列表（OpenAI格式）
	Tools []openaiSDK.Tool `json:"tools,omitempty" yaml:"tools,omitempty"`

	// ToolChoice 工具选择策略
	ToolChoice any `json:"toolChoice,omitempty" yaml:"toolChoice,omitempty"`

	// ResponseFormat 响应格式
	ResponseFormat *openaiSDK.ChatCompletionResponseFormat `json:"responseFormat,omitempty" yaml:"responseFormat,omitempty"`
}

// Function 函数调用定义
type Function struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

// ChatConfig 扩展的聊天配置，包含基础配置和推理参数
type ChatConfig struct {
	APIKey  string `json:"apiKey" yaml:"apiKey"`
	BaseURL string `json:"baseURL" yaml:"baseURL"`
	Model   string `json:"model" yaml:"model"`

	// 嵌入推理参数
	ModelParams `yaml:",inline" json:",inline"`
}

// ToPointer 辅助函数，用于将值转换为指针
func ToPointer[T any](value T) *T {
	return &value
}

// GetDefaultParams 获取默认参数
func GetDefaultParams() ModelParams {
	return ModelParams{
		Temperature:         ToPointer(float32(0.7)),
		TopP:                ToPointer(float32(0.9)),
		MaxCompletionTokens: ToPointer(4096),
		FrequencyPenalty:    ToPointer(float32(0.0)),
		PresencePenalty:     ToPointer(float32(0.0)),
		Stop:                []string{},
		Functions:           []Function{},
	}
}
