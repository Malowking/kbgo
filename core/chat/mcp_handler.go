package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/internal/mcp"
	"github.com/Malowking/kbgo/internal/mcp/client"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

// MCPHandler MCP tool call handler
type MCPHandler struct{}

// NewMCPHandler Create MCP handler
func NewMCPHandler() *MCPHandler {
	return &MCPHandler{}
}

// CallMCPToolsWithLLM 使用 LLM 智能选择并调用 MCP 工具
func (h *MCPHandler) CallMCPToolsWithLLM(ctx context.Context, req *v1.ChatReq) ([]*schema.Document, []*v1.MCPResult, error) {
	g.Log().Debugf(ctx, "Starting LLM intelligent tool call, question: %s", req.Question)

	// 创建 MCP 工具调用器
	toolCaller, err := mcp.NewMCPToolCaller(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("创建MCP工具调用器失败: %w", err)
	}
	defer toolCaller.Close()

	// 使用 LLM 智能选择并调用工具
	// 传递 MCPServiceTools 作为过滤器，限制 LLM 只能选择指定的工具
	documents, mcpResults, err := toolCaller.CallToolsWithLLM(ctx, req.ModelID, req.Question, req.ConvID, req.MCPServiceTools)
	if err != nil {
		return nil, nil, fmt.Errorf("LLM intelligent tool call failed: %w", err)
	}

	g.Log().Debugf(ctx, "LLM intelligent tool call completed, returned document count: %d, MCP result count: %d", len(documents), len(mcpResults))

	return documents, mcpResults, nil
}

// CallMCPTools 调用MCP工具（传统方式）
func (h *MCPHandler) CallMCPTools(ctx context.Context, req *v1.ChatReq) ([]*schema.Document, error) {
	g.Log().Debugf(ctx, "Starting MCP tool call, UseMCP: %v, MCPServiceTools: %v", req.UseMCP, req.MCPServiceTools)

	// Get all enabled MCP services
	registries, _, err := dao.MCPRegistry.List(ctx, nil, 1, 100)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to get MCP service list: %v", err)
		return nil, err
	}

	g.Log().Debugf(ctx, "Obtained %d MCP services", len(registries))

	// 如果没有注册的服务，直接返回
	if len(registries) == 0 {
		g.Log().Debug(ctx, "No registered MCP services found")
		return nil, nil
	}

	var documents []*schema.Document

	// 遍历所有MCP服务
	for _, registry := range registries {
		g.Log().Debugf(ctx, "Checking MCP service: %s, status: %d", registry.Name, registry.Status)

		// Check if service is enabled
		if registry.Status != 1 {
			g.Log().Debugf(ctx, "MCP service %s is not enabled, skipping", registry.Name)
			continue
		}

		// 处理服务的工具调用
		serviceDocs, err := h.processServiceTools(ctx, registry, req)
		if err != nil {
			g.Log().Errorf(ctx, "Failed to process tools for service %s: %v", registry.Name, err)
			continue
		}

		documents = append(documents, serviceDocs...)
	}

	g.Log().Debugf(ctx, "MCP tool call completed, returned document count: %d", len(documents))

	return documents, nil
}

// processServiceTools Process tool calls for a single service
func (h *MCPHandler) processServiceTools(ctx context.Context, registry *gormModel.MCPRegistry, req *v1.ChatReq) ([]*schema.Document, error) {
	// 创建客户端
	mcpClient := client.NewMCPClient(registry)
	g.Log().Debugf(ctx, "Creating MCP client: %s", registry.Name)

	// 初始化连接
	err := mcpClient.Initialize(ctx, map[string]interface{}{
		"name":    "kbgo",
		"version": "1.0.0",
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize MCP connection: %w", err)
	}
	g.Log().Debugf(ctx, "MCP connection initialized successfully: %s", registry.Name)

	// Get tool list
	tools, err := h.getServiceTools(ctx, registry, mcpClient)
	if err != nil {
		return nil, fmt.Errorf("Failed to get tool list: %w", err)
	}

	g.Log().Debugf(ctx, "Obtained %d MCP tools, tool list: %v", len(tools), tools)

	var documents []*schema.Document

	// 遍历工具并调用符合条件的工具
	for _, tool := range tools {
		g.Log().Debugf(ctx, "Checking tool: %s", tool.Name)

		// Check if the tool should be called
		if !h.shouldCallTool(ctx, registry.Name, tool.Name, req.MCPServiceTools) {
			continue
		}

		g.Log().Debugf(ctx, "Calling MCP tool: %s", tool.Name)

		// Call tool and get results
		doc, err := h.callTool(ctx, mcpClient, registry, tool, req)
		if err != nil {
			g.Log().Errorf(ctx, "Failed to call tool %s: %v", tool.Name, err)
			continue
		}

		if doc != nil {
			documents = append(documents, doc)
		}
	}

	return documents, nil
}

// getServiceTools Get tool list for a service
func (h *MCPHandler) getServiceTools(ctx context.Context, registry *gormModel.MCPRegistry, mcpClient *client.MCPClient) ([]client.MCPTool, error) {
	var tools []client.MCPTool

	// First try to get tool list from database cache
	if registry.Tools != "" && registry.Tools != "[]" {
		var toolInfos []v1.MCPToolInfo
		if err := json.Unmarshal([]byte(registry.Tools), &toolInfos); err == nil {
			// Convert to client.MCPTool format
			tools = make([]client.MCPTool, len(toolInfos))
			for i, info := range toolInfos {
				tools[i] = client.MCPTool{
					Name:        info.Name,
					Description: info.Description,
					InputSchema: info.InputSchema,
				}
			}
			g.Log().Debugf(ctx, "Obtained %d MCP tools from database cache", len(tools))
			return tools, nil
		}
	}

	// If no tool list in cache, get from remote service
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("Failed to get remote tool list: %w", err)
	}

	// Update tool list cache in database
	if len(tools) > 0 {
		h.updateToolsCache(ctx, registry, tools)
	}

	return tools, nil
}

// updateToolsCache Update tool cache
func (h *MCPHandler) updateToolsCache(ctx context.Context, registry *gormModel.MCPRegistry, tools []client.MCPTool) {
	// Convert to MCPToolInfo format
	toolInfos := make([]v1.MCPToolInfo, len(tools))
	for i, tool := range tools {
		toolInfos[i] = v1.MCPToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	// Serialize and save to database
	toolsJSON, err := json.Marshal(toolInfos)
	if err == nil {
		registry.Tools = string(toolsJSON)
		if updateErr := dao.MCPRegistry.Update(ctx, registry); updateErr != nil {
			g.Log().Errorf(ctx, "Failed to update MCP registry tools: %v", updateErr)
		}
	}
}

// shouldCallTool Check if tool should be called
func (h *MCPHandler) shouldCallTool(ctx context.Context, serviceName, toolName string, mcpServiceTools map[string][]string) bool {
	// Handle tool calling logic:
	// 1. If MCPServiceTools specifies tools for this service, only call specified tools
	// 2. If MCPServiceTools is empty or nil, call all tools

	if mcpServiceTools != nil {
		// Check if tools are specified for a specific service
		if serviceTools, exists := mcpServiceTools[serviceName]; exists {
			g.Log().Debugf(ctx, "Checking specified tool list for service %s: %v", serviceName, serviceTools)
			if len(serviceTools) == 0 {
				// Empty array means no tools for this service should be called
				g.Log().Debugf(ctx, "Service %s tool list is empty, skipping all tools", serviceName)
				return false
			}

			// Check if tool is in allowed list
			for i, allowedTool := range serviceTools {
				g.Log().Debugf(ctx, "Comparing tool names: index%d, '%s' vs '%s'", i, allowedTool, toolName)
				if allowedTool == toolName {
					g.Log().Debugf(ctx, "Found matching tool: %s", toolName)
					return true
				}
			}
			g.Log().Debugf(ctx, "Tool %s is not in the allowed list for service %s, skipping", toolName, serviceName)
			return false
		}
	} else {
		g.Log().Debug(ctx, "未指定工具列表，调用所有工具")
	}

	return true
}

// callTool 调用单个工具
func (h *MCPHandler) callTool(ctx context.Context, mcpClient *client.MCPClient, registry *gormModel.MCPRegistry, tool client.MCPTool, req *v1.ChatReq) (*schema.Document, error) {
	startTime := time.Now()

	// 智能参数映射：根据工具schema生成参数
	toolMapper := NewToolMapper()
	toolArgs, err := toolMapper.BuildToolArguments(tool, req.Question)
	if err != nil {
		g.Log().Warningf(ctx, "Failed to build tool parameters: %v", err)
		// 使用fallback策略
		toolArgs = toolMapper.FallbackToolArguments(tool, req.Question)
	}

	// 调用工具
	result, err := mcpClient.CallTool(ctx, tool.Name, toolArgs)

	// 计算耗时
	duration := int(time.Since(startTime).Milliseconds())

	// 记录调用日志
	h.logToolCall(ctx, registry, tool, req, toolArgs, result, err, duration)

	// If call fails, log error and return
	if err != nil {
		return nil, fmt.Errorf("Tool call failed: %w", err)
	}

	g.Log().Debugf(ctx, "MCP tool call successful: %s, result: %v", tool.Name, result)

	// 将结果转换为文档
	return h.convertResultToDocument(registry, tool, result)
}

// logToolCall 记录工具调用日志
func (h *MCPHandler) logToolCall(ctx context.Context, registry *gormModel.MCPRegistry, tool client.MCPTool, req *v1.ChatReq, toolArgs map[string]interface{}, result *client.MCPCallToolResult, err error, duration int) {
	// 序列化请求和响应
	reqPayload, _ := json.Marshal(toolArgs)
	respPayload, _ := json.Marshal(result)

	// 记录调用日志
	logStatus := int8(1) // Success
	errorMsg := ""
	if err != nil {
		logStatus = 0 // Failed
		errorMsg = err.Error()
	}

	logID := strings.ReplaceAll(uuid.New().String(), "-", "")
	callLog := &gormModel.MCPCallLog{
		ID:              logID,
		ConversationID:  req.ConvID,
		MCPRegistryID:   registry.ID,
		MCPServiceName:  registry.Name,
		ToolName:        tool.Name,
		RequestPayload:  string(reqPayload),
		ResponsePayload: string(respPayload),
		Status:          logStatus,
		ErrorMessage:    errorMsg,
		Duration:        duration,
	}

	if logErr := dao.MCPCallLog.Create(ctx, callLog); logErr != nil {
		g.Log().Errorf(ctx, "Failed to create MCP call log: %v", logErr)
	}
}

// convertResultToDocument Convert tool call result to document
func (h *MCPHandler) convertResultToDocument(registry *gormModel.MCPRegistry, tool client.MCPTool, result *client.MCPCallToolResult) (*schema.Document, error) {
	logID := strings.ReplaceAll(uuid.New().String(), "-", "")

	// 将结果转换为文档
	for _, content := range result.Content {
		if content.Type == "text" && content.Text != "" {
			doc := &schema.Document{
				ID:      logID,
				Content: content.Text,
				MetaData: map[string]interface{}{
					"source":    "mcp",
					"service":   registry.Name,
					"tool":      tool.Name,
					"tool_desc": tool.Description,
				},
			}
			return doc, nil
		}
	}

	return nil, nil
}

// ExtractMCPResults Extract result information from MCP documents
func (h *MCPHandler) ExtractMCPResults(docs []*schema.Document) []*v1.MCPResult {
	var results []*v1.MCPResult
	for _, doc := range docs {
		if source, ok := doc.MetaData["source"].(string); ok && source == "mcp" {
			result := &v1.MCPResult{
				ServiceName: "",
				ToolName:    "",
				Content:     doc.Content,
			}

			if serviceName, ok := doc.MetaData["service"].(string); ok {
				result.ServiceName = serviceName
			}
			if toolName, ok := doc.MetaData["tool"].(string); ok {
				result.ToolName = toolName
			}
			results = append(results, result)
		}
	}
	return results
}

// CallSingleTool 调用单个工具
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
