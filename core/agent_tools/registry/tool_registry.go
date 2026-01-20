package registry

import (
	"fmt"
	"sync"

	"github.com/sashabaranov/go-openai"
)

// ToolRegistry 工具定义注册表接口
type ToolRegistry interface {
	// RegisterTool 注册工具定义
	RegisterTool(name string, def *ToolDefinition) error

	// GetToolDefinition 获取工具定义
	GetToolDefinition(name string) (*ToolDefinition, error)

	// GetAllTools 获取所有工具定义
	GetAllTools() []*ToolDefinition

	// ConvertToOpenAITool 转换为 OpenAI Tool 格式
	ConvertToOpenAITool(name string) (*openai.Tool, error)

	// ConvertToOpenAITools 批量转换
	ConvertToOpenAITools(names []string) ([]openai.Tool, error)

	// HasTool 检查工具是否存在
	HasTool(name string) bool

	// UnregisterTool 注销工具
	UnregisterTool(name string) error
}

// ToolDefinition 工具定义
type ToolDefinition struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Parameters  *ParameterSchema `json:"parameters"`
	ToolType    string           `json:"tool_type"` // "local_tools", "mcp", "claude_skills"
}

// ParameterSchema 参数模式定义
type ParameterSchema struct {
	Type       string                         `json:"type"` // "object"
	Properties map[string]*PropertyDefinition `json:"properties"`
	Required   []string                       `json:"required,omitempty"`
}

// PropertyDefinition 属性定义
type PropertyDefinition struct {
	Type        string   `json:"type"`
	Description string   `json:"description"`
	Enum        []string `json:"enum,omitempty"` // 可选的枚举值
}

// memoryToolDefinitionRegistry 内存实现的工具定义注册表
type memoryToolDefinitionRegistry struct {
	mu    sync.RWMutex
	tools map[string]*ToolDefinition
}

// NewToolDefinitionRegistry 创建新的工具定义注册表
func NewToolDefinitionRegistry() ToolRegistry {
	return &memoryToolDefinitionRegistry{
		tools: make(map[string]*ToolDefinition),
	}
}

// RegisterTool 注册工具定义
func (r *memoryToolDefinitionRegistry) RegisterTool(name string, def *ToolDefinition) error {
	if name == "" {
		return fmt.Errorf("tool name cannot be empty")
	}
	if def == nil {
		return fmt.Errorf("tool definition cannot be nil")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已存在
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %s already registered", name)
	}

	r.tools[name] = def
	return nil
}

// GetToolDefinition 获取工具定义
func (r *memoryToolDefinitionRegistry) GetToolDefinition(name string) (*ToolDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	def, exists := r.tools[name]
	if !exists {
		return nil, fmt.Errorf("tool %s not found", name)
	}

	return def, nil
}

// GetAllTools 获取所有工具定义
func (r *memoryToolDefinitionRegistry) GetAllTools() []*ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]*ToolDefinition, 0, len(r.tools))
	for _, def := range r.tools {
		tools = append(tools, def)
	}

	return tools
}

// ConvertToOpenAITool 转换为 OpenAI Tool 格式
func (r *memoryToolDefinitionRegistry) ConvertToOpenAITool(name string) (*openai.Tool, error) {
	def, err := r.GetToolDefinition(name)
	if err != nil {
		return nil, err
	}

	return convertToolDefinitionToOpenAI(def)
}

// ConvertToOpenAITools 批量转换
func (r *memoryToolDefinitionRegistry) ConvertToOpenAITools(names []string) ([]openai.Tool, error) {
	tools := make([]openai.Tool, 0, len(names))

	for _, name := range names {
		tool, err := r.ConvertToOpenAITool(name)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tool %s: %w", name, err)
		}
		tools = append(tools, *tool)
	}

	return tools, nil
}

// HasTool 检查工具是否存在
func (r *memoryToolDefinitionRegistry) HasTool(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.tools[name]
	return exists
}

// UnregisterTool 注销工具
func (r *memoryToolDefinitionRegistry) UnregisterTool(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool %s not found", name)
	}

	delete(r.tools, name)
	return nil
}

// convertToolDefinitionToOpenAI 将工具定义转换为 OpenAI 格式
func convertToolDefinitionToOpenAI(def *ToolDefinition) (*openai.Tool, error) {
	// 构建参数 JSON
	params := make(map[string]interface{})
	params["type"] = def.Parameters.Type

	// 转换 properties
	properties := make(map[string]interface{})
	for propName, propDef := range def.Parameters.Properties {
		prop := map[string]interface{}{
			"type":        propDef.Type,
			"description": propDef.Description,
		}
		if len(propDef.Enum) > 0 {
			prop["enum"] = propDef.Enum
		}
		properties[propName] = prop
	}
	params["properties"] = properties

	// 添加 required 字段
	if len(def.Parameters.Required) > 0 {
		params["required"] = def.Parameters.Required
	}

	return &openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  params,
		},
	}, nil
}
