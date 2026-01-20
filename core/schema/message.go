package schema

// RoleType 消息角色类型
type RoleType string

const (
	System    RoleType = "system"
	User      RoleType = "user"
	Assistant RoleType = "assistant"
	Tool      RoleType = "tool"
)

// Message 表示对话消息
type Message struct {
	// Role 消息角色：system, user, assistant, tool
	Role RoleType `json:"role"`
	// Content 文本内容
	Content string `json:"content,omitempty"`

	// ReasoningContent 思考内容（用于思考模型）
	ReasoningContent string `json:"reasoning_content,omitempty"`

	// UserInputMultiContent 用户多模态输入内容
	UserInputMultiContent []MessageInputPart `json:"user_input_multi_content,omitempty"`

	// ToolCalls 工具调用列表（Assistant消息使用）
	ToolCalls []ToolCall `json:"tool_calls,omitempty"`

	// ToolCallID 工具调用ID（Tool消息使用）
	ToolCallID string `json:"tool_call_id,omitempty"`

	// Extra 扩展字段，用于存储额外信息
	Extra map[string]any `json:"extra,omitempty"`
}

// MessageInputPart 消息输入部分
type MessageInputPart struct {
	Type  MessagePartType    `json:"type"`
	Text  string             `json:"text,omitempty"`
	Image *MessageInputImage `json:"image,omitempty"`
	Audio *MessageInputAudio `json:"audio,omitempty"`
	Video *MessageInputVideo `json:"video,omitempty"`
}

// MessagePartCommon 消息部分的公共字段
type MessagePartCommon struct {
	URL        *string `json:"url,omitempty"`
	Base64Data *string `json:"base64_data,omitempty"`
	MIMEType   string  `json:"mime_type,omitempty"`
}

// MessageInputImage 图片输入
type MessageInputImage struct {
	MessagePartCommon
	Detail string `json:"detail,omitempty"` // auto, low, high
}

// MessageInputAudio 音频输入
type MessageInputAudio struct {
	MessagePartCommon
}

// MessageInputVideo 视频输入
type MessageInputVideo struct {
	MessagePartCommon
}

// MessagePartType 消息部分类型
type MessagePartType string

const (
	MessagePartTypeText     MessagePartType = "text"
	MessagePartTypeImageURL MessagePartType = "image_url"
	MessagePartTypeAudioURL MessagePartType = "audio_url"
	MessagePartTypeVideoURL MessagePartType = "video_url"
)

// ImageDetailLevel 图片详细程度
const (
	ImageDetailAuto string = "auto"
	ImageDetailLow  string = "low"
	ImageDetailHigh string = "high"
)

// ToolCall 工具调用
type ToolCall struct {
	// Index 在多个工具调用中的索引（流式模式使用）
	Index *int `json:"index,omitempty"`
	// ID 工具调用的唯一标识
	ID string `json:"id"`
	// Type 工具调用类型，默认为 "function"
	Type string `json:"type"`
	// Function 要调用的函数
	Function FunctionCall `json:"function"`
}

// FunctionCall 函数调用
type FunctionCall struct {
	// Name 函数名称
	Name string `json:"name"`
	// Arguments 函数参数（JSON字符串）
	Arguments string `json:"arguments"`
}

// ToolInfo 工具信息
type ToolInfo struct {
	// Name 工具名称
	Name string
	// Desc 工具描述
	Desc string
	// ParamsOneOf 参数定义
	ParamsOneOf *ParamsOneOf
}

// ParamsOneOf 参数定义（可以是多种格式之一）
type ParamsOneOf struct {
	// params 参数映射
	params map[string]*ParameterInfo
}

// NewParamsOneOfByParams 通过参数映射创建 ParamsOneOf
func NewParamsOneOfByParams(params map[string]*ParameterInfo) *ParamsOneOf {
	return &ParamsOneOf{
		params: params,
	}
}

// ToOpenAPIV3 转换为 OpenAPI v3 格式（简化版）
func (p *ParamsOneOf) ToOpenAPIV3() (interface{}, error) {
	if p == nil || p.params == nil {
		return nil, nil
	}

	properties := make(map[string]interface{})
	required := []string{}

	for name, param := range p.params {
		properties[name] = map[string]interface{}{
			"type":        param.Type,
			"description": param.Desc,
		}
		if param.Required {
			required = append(required, name)
		}
	}

	result := map[string]interface{}{
		"type":       "object",
		"properties": properties,
	}

	if len(required) > 0 {
		result["required"] = required
	}

	return result, nil
}

// ParameterInfo 参数信息
type ParameterInfo struct {
	// Type 参数类型：string, number, boolean, object, array
	Type string
	// Desc 参数描述
	Desc string
	// Required 是否必需
	Required bool
}
