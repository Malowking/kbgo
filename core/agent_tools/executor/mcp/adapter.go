package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/schema"
)

// MCPAdapter MCP 结果适配器
// 职责：将 MCP 响应转换为统一的 ToolResult
type MCPAdapter struct{}

// NewMCPAdapter 创建 MCP 适配器
func NewMCPAdapter() *MCPAdapter {
	return &MCPAdapter{}
}

// ToToolResult 将 MCP 结果转换为 ToolResult
func (a *MCPAdapter) ToToolResult(
	mcpResult *MCPCallToolResult,
	serviceName, toolName string,
	duration time.Duration,
) (*schema.ToolResult, error) {
	if mcpResult == nil {
		return schema.NewToolResultWithError(
			schema.ErrCodeInternal,
			"MCP结果为空",
			"",
		), nil
	}

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

	return result, nil
}
