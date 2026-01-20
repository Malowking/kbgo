package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Malowking/kbgo/core/agent_tools/registry"
	"github.com/gogf/gf/v2/frame/g"
)

// RegisterMCPTools 从 MCP 客户端动态获取工具定义并注册
func RegisterMCPTools(ctx context.Context, reg registry.ToolRegistry, mcpClient *MCPClient, serviceName string) error {
	g.Log().Infof(ctx, "[MCP] Registering tools from service: %s", serviceName)

	// 获取 MCP 工具列表
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to list MCP tools: %w", err)
	}

	g.Log().Infof(ctx, "[MCP] Found %d tools from service %s", len(tools), serviceName)

	// 注册每个工具
	for _, tool := range tools {
		// 转换参数定义
		params, err := convertMCPSchemaToParameterSchema(tool.InputSchema)
		if err != nil {
			g.Log().Warningf(ctx, "[MCP] Failed to convert schema for tool %s: %v", tool.Name, err)
			continue
		}

		// 构建工具名称（添加 mcp_ 前缀和服务名避免冲突）
		// 格式: mcp_<serviceName>__<toolName>
		toolName := fmt.Sprintf("mcp_%s__%s", serviceName, tool.Name)

		// 注册工具
		toolDef := &registry.ToolDefinition{
			Name:        toolName,
			Description: tool.Description,
			ToolType:    "mcp",
			Parameters:  params,
		}

		if err := reg.RegisterTool(toolName, toolDef); err != nil {
			g.Log().Warningf(ctx, "[MCP] Failed to register tool %s: %v", toolName, err)
			continue
		}

		g.Log().Infof(ctx, "[MCP] Registered tool: %s", toolName)
	}

	return nil
}

// convertMCPSchemaToParameterSchema 将 MCP 的 inputSchema 转换为统一的 ParameterSchema
func convertMCPSchemaToParameterSchema(inputSchema map[string]interface{}) (*registry.ParameterSchema, error) {
	// MCP 的 inputSchema 通常是 JSON Schema 格式
	// 示例:
	// {
	//   "type": "object",
	//   "properties": {
	//     "query": {
	//       "type": "string",
	//       "description": "Search query"
	//     }
	//   },
	//   "required": ["query"]
	// }

	params := &registry.ParameterSchema{
		Type:       "object",
		Properties: make(map[string]*registry.PropertyDefinition),
		Required:   []string{},
	}

	// 提取 type
	if schemaType, ok := inputSchema["type"].(string); ok {
		params.Type = schemaType
	}

	// 提取 properties
	if properties, ok := inputSchema["properties"].(map[string]interface{}); ok {
		for propName, propValue := range properties {
			propMap, ok := propValue.(map[string]interface{})
			if !ok {
				continue
			}

			propDef := &registry.PropertyDefinition{}

			// 提取 type
			if propType, ok := propMap["type"].(string); ok {
				propDef.Type = propType
			}

			// 提取 description
			if propDesc, ok := propMap["description"].(string); ok {
				propDef.Description = propDesc
			}

			// 提取 enum（如果有）
			if enumValue, ok := propMap["enum"].([]interface{}); ok {
				propDef.Enum = make([]string, 0, len(enumValue))
				for _, e := range enumValue {
					if enumStr, ok := e.(string); ok {
						propDef.Enum = append(propDef.Enum, enumStr)
					}
				}
			}

			params.Properties[propName] = propDef
		}
	}

	// 提取 required
	if required, ok := inputSchema["required"].([]interface{}); ok {
		params.Required = make([]string, 0, len(required))
		for _, r := range required {
			if reqStr, ok := r.(string); ok {
				params.Required = append(params.Required, reqStr)
			}
		}
	}

	return params, nil
}

// ConvertMCPToolsToOpenAI 将 MCP 工具列表转换为 OpenAI Tool 格式（辅助函数）
func ConvertMCPToolsToOpenAI(tools []MCPTool, serviceName string) ([]map[string]interface{}, error) {
	result := make([]map[string]interface{}, 0, len(tools))

	for _, tool := range tools {
		// 构建 OpenAI Tool 格式
		openaiTool := map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        fmt.Sprintf("mcp_%s__%s", serviceName, tool.Name),
				"description": tool.Description,
				"parameters":  tool.InputSchema,
			},
		}

		result = append(result, openaiTool)
	}

	return result, nil
}

// GetMCPToolDefinition 从 MCP 工具创建工具定义（辅助函数）
func GetMCPToolDefinition(tool MCPTool, serviceName string) (*registry.ToolDefinition, error) {
	params, err := convertMCPSchemaToParameterSchema(tool.InputSchema)
	if err != nil {
		return nil, fmt.Errorf("failed to convert schema: %w", err)
	}

	toolName := fmt.Sprintf("mcp_%s__%s", serviceName, tool.Name)

	return &registry.ToolDefinition{
		Name:        toolName,
		Description: tool.Description,
		ToolType:    "mcp",
		Parameters:  params,
	}, nil
}

// ValidateMCPToolSchema 验证 MCP 工具的 schema 是否有效
func ValidateMCPToolSchema(inputSchema map[string]interface{}) error {
	// 检查必需字段
	if _, ok := inputSchema["type"]; !ok {
		return fmt.Errorf("missing 'type' field in schema")
	}

	// 如果是 object 类型，检查 properties
	if schemaType, ok := inputSchema["type"].(string); ok && schemaType == "object" {
		if _, ok := inputSchema["properties"]; !ok {
			return fmt.Errorf("object type schema must have 'properties' field")
		}
	}

	return nil
}

// SerializeMCPToolDefinition 序列化 MCP 工具定义为 JSON（用于存储）
func SerializeMCPToolDefinition(tool MCPTool, serviceName string) (string, error) {
	toolDef, err := GetMCPToolDefinition(tool, serviceName)
	if err != nil {
		return "", err
	}

	data, err := json.Marshal(toolDef)
	if err != nil {
		return "", fmt.Errorf("failed to marshal tool definition: %w", err)
	}

	return string(data), nil
}

// DeserializeMCPToolDefinition 从 JSON 反序列化 MCP 工具定义
func DeserializeMCPToolDefinition(data string) (*registry.ToolDefinition, error) {
	var toolDef registry.ToolDefinition
	if err := json.Unmarshal([]byte(data), &toolDef); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tool definition: %w", err)
	}

	return &toolDef, nil
}
