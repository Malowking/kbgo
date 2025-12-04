package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/mcp"
	"github.com/Malowking/kbgo/internal/mcp/client"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// MCPHandler MCP tool call handler
type MCPHandler struct{}

// NewMCPHandler Create MCP handler
func NewMCPHandler() *MCPHandler {
	return &MCPHandler{}
}

// CallMCPToolsWithLLM 使用 LLM 智能选择并调用 MCP 工具
// documents: 知识检索的结果
// fileContent: 文件解析的文本内容
func (h *MCPHandler) CallMCPToolsWithLLM(ctx context.Context, req *v1.ChatReq, documents []*schema.Document, fileContent string) ([]*schema.Document, []*v1.MCPResult, error) {
	g.Log().Debugf(ctx, "Starting LLM intelligent tool call, question: %s", req.Question)

	// 创建 MCP 工具调用器
	toolCaller, err := mcp.NewMCPToolCaller(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("创建MCP工具调用器失败: %w", err)
	}
	defer toolCaller.Close()

	// 构建完整的用户问题（包含知识检索和文件解析的结果）
	fullQuestion := h.buildFullQuestion(ctx, req.Question, documents, fileContent)

	// 使用 LLM 智能选择并调用工具
	// 传递 MCPServiceTools 作为过滤器，限制 LLM 只能选择指定的工具
	mcpDocuments, mcpResults, err := toolCaller.CallToolsWithLLM(ctx, req.ModelID, fullQuestion, req.ConvID, req.MCPServiceTools)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM intelligent tool call failed: %w", err)
	}

	g.Log().Debugf(ctx, "LLM intelligent tool call completed, returned document count: %d, MCP result count: %d", len(mcpDocuments), len(mcpResults))

	return mcpDocuments, mcpResults, nil
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
		return nil, nil, fmt.Errorf("Failed to get MCP service: %w", err)
	}

	// 创建客户端
	mcpClient := client.NewMCPClient(registry)

	// 初始化连接
	err = mcpClient.Initialize(ctx, map[string]interface{}{
		"name":    "kbgo",
		"version": "1.0.0",
	})
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to initialize MCP connection: %w", err)
	}

	// 调用工具
	result, err := mcpClient.CallTool(ctx, toolName, args)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to call tool: %w", err)
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

// CallMCPToolsWithLLMAndSave Use LLM to intelligently select and call MCP tools, and save message history
func (h *MCPHandler) CallMCPToolsWithLLMAndSave(ctx context.Context, modelID string, convID string, messages []*schema.Message, llmTools []*schema.ToolInfo) ([]*schema.Document, []*v1.MCPResult, error) {
	// 1. 创建 MCP 工具调用器
	toolCaller, err := mcp.NewMCPToolCaller(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("创建MCP工具调用器失败: %w", err)
	}
	defer toolCaller.Close()

	// 2. Save initial message history
	chatInstance := chat.GetChat()
	// Note: There is no SaveMessages method here, we will save the response in GenerateWithToolsAndSave

	// 3. 调用 LLM（最多循环 5 次以支持多轮工具调用）
	maxIterations := 5
	var allDocuments []*schema.Document
	var allMCPResults []*v1.MCPResult
	var finalAnswer string                    // 保存 LLM 的最终文本回答
	var toolCallLogs []map[string]interface{} // 记录工具调用日志

	for iteration := 0; iteration < maxIterations; iteration++ {
		g.Log().Debugf(ctx, "====== 工具调用迭代 %d/%d ======", iteration+1, maxIterations)

		// 调用 LLM
		response, err := chatInstance.GenerateWithTools(ctx, modelID, messages, llmTools)
		if err != nil {
			return nil, nil, fmt.Errorf("LLM 调用失败: %w", err)
		}

		g.Log().Debugf(ctx, "LLM 响应 - Content: %s, ToolCalls数量: %d", response.Content, len(response.ToolCalls))

		// 将 LLM 响应添加到消息历史
		messages = append(messages, response)

		// 注意：GenerateWithToolsAndSave已经保存了消息，这里不需要再保存

		// 4. 检查是否有工具调用
		if len(response.ToolCalls) == 0 {
			// 没有工具调用，LLM 已经给出最终答案
			finalAnswer = response.Content
			g.Log().Infof(ctx, "LLM 未调用任何工具，给出最终答案（长度: %d）", len(finalAnswer))
			break
		}

		// 5. 执行所有工具调用
		g.Log().Infof(ctx, "LLM 要求调用 %d 个工具", len(response.ToolCalls))

		for idx, toolCall := range response.ToolCalls {
			// 解析工具名（格式：serviceName__toolName）
			serviceName, toolName := client.ParseToolName(toolCall.Function.Name)
			g.Log().Debugf(ctx, "[工具 %d/%d] 调用: %s (服务: %s, 工具: %s)",
				idx+1, len(response.ToolCalls), toolCall.Function.Name, serviceName, toolName)

			// 解析参数
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				errMsg := fmt.Sprintf("参数解析错误: %v", err)
				g.Log().Errorf(ctx, "[工具 %d/%d] %s", idx+1, len(response.ToolCalls), errMsg)

				// 添加错误响应到消息历史
				messages = append(messages, &schema.Message{
					Role:       schema.Tool, // 注意：这里应该是 Tool 而不是 "tool"
					Content:    errMsg,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			g.Log().Debugf(ctx, "[工具 %d/%d] 参数: %v", idx+1, len(response.ToolCalls), args)

			// 调用工具
			result, mcpResult, err := h.CallSingleTool(ctx, serviceName, toolName, args, convID)
			if err != nil {
				errMsg := fmt.Sprintf("工具调用失败: %v", err)
				g.Log().Errorf(ctx, "[工具 %d/%d] %s", idx+1, len(response.ToolCalls), errMsg)

				// 添加错误响应到消息历史
				messages = append(messages, &schema.Message{
					Role:       schema.Tool,
					Content:    errMsg,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			g.Log().Debugf(ctx, "[工具 %d/%d] 执行成功，结果长度: %d",
				idx+1, len(response.ToolCalls), len(result.Content))

			// 收集结果
			if result != nil {
				allDocuments = append(allDocuments, result)
			}
			if mcpResult != nil {
				allMCPResults = append(allMCPResults, mcpResult)
			}

			// 记录工具调用日志
			toolCallLog := map[string]interface{}{
				"service_name": serviceName,
				"tool_name":    toolName,
				"arguments":    args,
				"result":       mcpResult.Content,
			}
			toolCallLogs = append(toolCallLogs, toolCallLog)

			// 【关键】将工具执行结果添加到消息历史，供 LLM 下次调用时使用
			toolResultMsg := &schema.Message{
				Role:       schema.Tool, // 注意：这里应该是 Tool 而不是 "tool"
				Content:    mcpResult.Content,
				ToolCallID: toolCall.ID,
			}
			messages = append(messages, toolResultMsg)

			g.Log().Debugf(ctx, "[工具 %d/%d] 结果已添加到消息历史", idx+1, len(response.ToolCalls))
		}

		// 如果这是最后一次迭代，需要再调用一次 LLM 让它基于工具结果给出最终答案
		if iteration == maxIterations-1 {
			g.Log().Warning(ctx, "达到最大工具调用迭代次数，尝试获取最终答案")

			// 最后一次调用 LLM，不再提供工具（强制它给出最终答案）
			finalResponse, err := chatInstance.GenerateWithTools(ctx, modelID, messages, nil)
			if err != nil {
				g.Log().Errorf(ctx, "获取最终答案失败: %v", err)
			} else {
				finalAnswer = finalResponse.Content
				// 注意：GenerateWithToolsAndSave已经保存了消息，这里不需要再保存
				g.Log().Debugf(ctx, "获取到最终答案（长度: %d）", len(finalAnswer))
			}
			break
		}
	}

	// 6. 返回结果
	return allDocuments, allMCPResults, nil
}
