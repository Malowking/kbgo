package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/mcp"
	"github.com/Malowking/kbgo/internal/mcp/client"
	"github.com/Malowking/kbgo/pkg/schema"
)

// MCPHandler MCP tool call handler
type MCPHandler struct{}

// NewMCPHandler Create MCP handler
func NewMCPHandler() *MCPHandler {
	return &MCPHandler{}
}

// CallMCPToolsWithLLM 使用 LLM 智能选择并调用 MCP 工具
func (h *MCPHandler) CallMCPToolsWithLLM(ctx context.Context, req *v1.ChatReq, documents []*schema.Document, fileContent string) ([]*schema.Document, []*v1.MCPResult, string, error) {
	// 创建 MCP 工具调用器
	toolCaller, err := mcp.NewMCPToolCaller(ctx)
	if err != nil {
		return nil, nil, "", fmt.Errorf("创建MCP工具调用器失败: %w", err)
	}
	defer toolCaller.Close()

	// 构建完整的用户问题（包含知识检索和文件解析的结果）
	fullQuestion := h.buildFullQuestion(ctx, req.Question, documents, fileContent)

	// 使用 LLM 智能选择并调用工具
	mcpDocuments, mcpResults, finalAnswer, err := toolCaller.CallToolsWithLLM(ctx, req.ModelID, fullQuestion, req.ConvID, req.MCPServiceTools)
	if err != nil {
		return nil, nil, "", fmt.Errorf("LLM intelligent tool call failed: %w", err)
	}

	return mcpDocuments, mcpResults, finalAnswer, nil
}

// buildFullQuestion 构建包含知识检索和文件解析结果的完整问题
func (h *MCPHandler) buildFullQuestion(ctx context.Context, question string, documents []*schema.Document, fileContent string) string {
	var builder strings.Builder

	builder.WriteString(question)

	// 如果有知识库检索内容，添加到问题中
	if len(documents) > 0 {
		builder.WriteString("\n\n【知识库检索结果】\n")
		for i, doc := range documents {
			builder.WriteString(fmt.Sprintf("文档%d: %s\n", i+1, doc.Content))
		}
	}

	// 如果有文件解析内容，添加到问题中
	if fileContent != "" {
		builder.WriteString("\n\n【文件解析内容】\n")
		builder.WriteString(fileContent)
	}

	return builder.String()
}

// CallSingleTool 调用单个工具（由LLM决定参数）
func (h *MCPHandler) CallSingleTool(ctx context.Context, serviceName string, toolName string, args map[string]interface{}, convID string) (*schema.Document, *v1.MCPResult, error) {
	// Get MCP service
	registry, err := dao.MCPRegistry.GetByName(ctx, serviceName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get MCP service: %w", err)
	}

	// 创建客户端
	mcpClient := client.NewMCPClient(registry)

	// 初始化连接
	err = mcpClient.Initialize(ctx, map[string]interface{}{
		"name":    "kbgo",
		"version": "1.0.0",
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to initialize MCP connection: %w", err)
	}

	// 调用工具
	result, err := mcpClient.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to call tool: %w", err)
	}

	// 提取文本内容
	var content string
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			content += c.Text + "\n"
		}
	}
	content = strings.TrimSpace(content)

	// 将结果转换为文档
	doc := &schema.Document{
		ID:      "mcp_" + serviceName + "_" + toolName,
		Content: content,
		MetaData: map[string]interface{}{
			"source":    "mcp",
			"service":   serviceName,
			"tool":      toolName,
			"tool_desc": "", // result.Description 不存在
		},
	}

	// 将结果转换为MCPResult
	mcpResult := &v1.MCPResult{
		ServiceName: serviceName,
		ToolName:    toolName,
		Content:     content,
	}

	return doc, mcpResult, nil
}
