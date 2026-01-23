package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/agent_tools/framework"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// MCPToolExecutor MCP 工具执行器
type MCPToolExecutor struct {
	// clientManager MCP 客户端管理器
	clientManager *MCPClient
}

// NewMCPToolExecutor 创建 MCP 工具执行器
func NewMCPToolExecutor(client *MCPClient) *MCPToolExecutor {
	return &MCPToolExecutor{
		clientManager: client,
	}
}

// Execute 执行 MCP 工具
func (e *MCPToolExecutor) Execute(
	ctx framework.ToolContext,
	tool framework.ToolDefinition,
	input map[string]interface{},
) (*schema.ToolResult, error) {
	startTime := time.Now()

	// 解析工具名（格式：mcp_service_tool）
	serviceName, toolName := parseToolName(tool.Name)
	if serviceName == "" || toolName == "" {
		return schema.NewToolResultFromError(
			schema.NewValidationError(
				"无效的MCP工具名称格式",
				fmt.Sprintf("期望格式: mcp_service_tool, 实际: %s", tool.Name),
			),
		), nil
	}

	g.Log().Infof(ctx.Context, "[MCP执行器] 调用 %s.%s", serviceName, toolName)

	// 调用 MCP 工具
	result, err := e.clientManager.CallTool(ctx.Context, toolName, input)
	if err != nil {
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("MCP工具调用失败: %v", err), ""),
		), nil
	}

	// 转换为 ToolResult
	toolResult := adaptMCPResultToToolResult(result, serviceName, toolName, time.Since(startTime))

	return toolResult, nil
}

// parseToolName 解析工具名（格式：mcp_service_tool）
func parseToolName(fullName string) (serviceName, toolName string) {
	if strings.HasPrefix(fullName, "mcp_") {
		trimmed := strings.TrimPrefix(fullName, "mcp_")
		parts := strings.SplitN(trimmed, "__", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		parts = strings.SplitN(trimmed, "_", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "", ""
	}

	parts := strings.SplitN(fullName, "__", 2)
	if len(parts) != 2 {
		return "", ""
	}
	return parts[0], parts[1]
}

// adaptMCPResultToToolResult 将 MCP 结果转换为 ToolResult
func adaptMCPResultToToolResult(
	mcpResult *MCPCallToolResult,
	serviceName, toolName string,
	duration time.Duration,
) *schema.ToolResult {
	// 提取文本内容
	var content strings.Builder
	var artifacts []*schema.Artifact

	for _, c := range mcpResult.Content {
		switch c.Type {
		case "text":
			if c.Text != "" {
				content.WriteString(c.Text)
				content.WriteString("\n")
			}
		case "image":
			// 处理图片类型
			if c.Data != "" {
				// 创建图片附件
				artifact := &schema.Artifact{
					Type: "image",
					URL:  c.Data, // base64 或 URL
					// MimeType: c.MimeType,
					Name: fmt.Sprintf("%s_%s_image", serviceName, toolName),
				}
				artifacts = append(artifacts, artifact)
			}
		case "resource":
			// 处理资源类型
			// Resource field not available in MCPContent
		}
	}

	// 创建成功的 ToolResult
	result := schema.NewToolResult(strings.TrimSpace(content.String()))

	// 添加附件
	for _, artifact := range artifacts {
		result.WithArtifact(artifact)
	}

	// 添加元数据
	result.WithMetadata("source", "mcp")
	result.WithMetadata("service", serviceName)
	result.WithMetadata("tool", toolName)

	// 添加执行指标
	result.WithMetrics(schema.NewExecutionMetrics(duration, 0, 0))

	return result
}
