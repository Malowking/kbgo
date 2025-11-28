package chat

import (
	"fmt"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// ModelParams 封装大模型的推理参数
type ModelParams struct {
	// Temperature 控制输出的随机性，0.0-2.0，越小越确定性
	Temperature *float64 `json:"temperature,omitempty" yaml:"temperature,omitempty"`

	// TopP 核采样参数，0.0-1.0，控制生成的多样性
	TopP *float64 `json:"topP,omitempty" yaml:"topP,omitempty"`

	// MaxTokens 最大生成token数量
	MaxTokens *int `json:"maxTokens,omitempty" yaml:"maxTokens,omitempty"`

	// FrequencyPenalty 频率惩罚，减少重复词汇，-2.0到2.0
	FrequencyPenalty *float64 `json:"frequencyPenalty,omitempty" yaml:"frequencyPenalty,omitempty"`

	// PresencePenalty 存在惩罚，鼓励新话题，-2.0到2.0
	PresencePenalty *float64 `json:"presencePenalty,omitempty" yaml:"presencePenalty,omitempty"`

	// Stop 停止词列表
	Stop []string `json:"stop,omitempty" yaml:"stop,omitempty"`

	// Functions 函数调用配置
	Functions []Function `json:"functions,omitempty" yaml:"functions,omitempty"`
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
		Temperature:      ToPointer(0.7),
		TopP:             ToPointer(0.9),
		MaxTokens:        ToPointer(4096),
		FrequencyPenalty: ToPointer(0.0),
		PresencePenalty:  ToPointer(0.0),
		Stop:             []string{},
		Functions:        []Function{},
	}
}

// ToModelOptions 将 ModelParams 转换为 eino model.Option 列表
func (p ModelParams) ToModelOptions() []model.Option {
	var options []model.Option

	// 基础参数 - 使用eino标准选项
	if p.Temperature != nil {
		options = append(options, model.WithTemperature(float32(*p.Temperature)))
	}
	if p.TopP != nil {
		options = append(options, model.WithTopP(float32(*p.TopP)))
	}
	if p.MaxTokens != nil {
		options = append(options, model.WithMaxTokens(*p.MaxTokens))
	}
	if len(p.Stop) > 0 {
		options = append(options, model.WithStop(p.Stop))
	}

	// 函数调用支持
	if len(p.Functions) > 0 {
		tools := make([]*schema.ToolInfo, len(p.Functions))
		for i, fn := range p.Functions {
			// 将函数参数转换为ParameterInfo map
			params := make(map[string]*schema.ParameterInfo)
			if fn.Parameters != nil {
				// 这里需要根据实际的参数结构进行转换
				// 简化实现：假设Parameters是一个map[string]interface{}格式
				for key, value := range fn.Parameters {
					paramInfo := &schema.ParameterInfo{
						Type:     schema.Object, // 可以根据实际类型动态设置
						Desc:     fmt.Sprintf("Parameter: %s", key),
						Required: false,
					}
					// 如果value也是map，表示有更复杂的结构
					if _, ok := value.(map[string]interface{}); ok {
						paramInfo.Type = schema.Object
					}
					params[key] = paramInfo
				}
			}

			tools[i] = &schema.ToolInfo{
				Name:        fn.Name,
				Desc:        fn.Description,
				ParamsOneOf: schema.NewParamsOneOfByParams(params),
			}
		}
		options = append(options, model.WithTools(tools))
	}

	// OpenAI特定参数 - 使用ExtraFields传递
	extraFields := make(map[string]interface{})
	if p.FrequencyPenalty != nil {
		extraFields["frequency_penalty"] = *p.FrequencyPenalty
	}
	if p.PresencePenalty != nil {
		extraFields["presence_penalty"] = *p.PresencePenalty
	}

	if len(extraFields) > 0 {
		options = append(options, openai.WithExtraFields(extraFields))
	}

	return options
}
