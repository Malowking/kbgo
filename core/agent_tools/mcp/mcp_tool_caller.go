package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools/mcp/client"
	"github.com/Malowking/kbgo/core/errors"
	internalCache "github.com/Malowking/kbgo/internal/cache"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/chat"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

// MCPServiceClient MCP 服务客户端封装
type MCPServiceClient struct {
	Registry *gormModel.MCPRegistry
	Client   *client.MCPClient
	Tools    []client.MCPTool
}

// MCPToolCaller MCP 工具调用器
type MCPToolCaller struct {
	services map[string]*MCPServiceClient // 服务名 -> 服务客户端
}

// NewMCPToolCaller 创建 MCP 工具调用器
func NewMCPToolCaller(ctx context.Context) (*MCPToolCaller, error) {
	// 获取所有启用的MCP服务
	registries, _, err := dao.MCPRegistry.List(ctx, nil, 1, 100)
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "获取MCP服务列表失败: %v", err)
	}

	services := make(map[string]*MCPServiceClient)

	// 初始化每个服务
	for _, registry := range registries {
		if registry.Status != 1 {
			continue // 跳过未启用的服务
		}

		mcpClient := client.NewMCPClient(registry)

		// 初始化连接
		err = mcpClient.Initialize(ctx, map[string]interface{}{
			"name":    "kbgo",
			"version": "1.0.0",
		})
		if err != nil {
			g.Log().Errorf(ctx, "Failed to initialize MCP service %s: %v", registry.Name, err)
			continue
		}

		// 获取工具列表
		var tools []client.MCPTool

		// 首先尝试从数据库缓存中获取
		if registry.Tools != "" && registry.Tools != "[]" {
			var toolInfos []v1.MCPToolInfo
			if err := json.Unmarshal([]byte(registry.Tools), &toolInfos); err == nil {
				tools = make([]client.MCPTool, len(toolInfos))
				for i, info := range toolInfos {
					tools[i] = client.MCPTool{
						Name:        info.Name,
						Description: info.Description,
						InputSchema: info.InputSchema,
					}
				}
			}
		}

		// 如果缓存中没有，从远程获取
		if len(tools) == 0 {
			tools, err = mcpClient.ListTools(ctx)
			if err != nil {
				g.Log().Errorf(ctx, "Failed to list tools for service %s: %v", registry.Name, err)
				continue
			}

			// 更新缓存
			if len(tools) > 0 {
				toolInfos := make([]v1.MCPToolInfo, len(tools))
				for i, tool := range tools {
					toolInfos[i] = v1.MCPToolInfo{
						Name:        tool.Name,
						Description: tool.Description,
						InputSchema: tool.InputSchema,
					}
				}
				toolsJSON, _ := json.Marshal(toolInfos)
				registry.Tools = string(toolsJSON)
				dao.MCPRegistry.Update(ctx, registry)
			}
		}

		services[registry.Name] = &MCPServiceClient{
			Registry: registry,
			Client:   mcpClient,
			Tools:    tools,
		}
	}

	return &MCPToolCaller{
		services: services,
	}, nil
}

// GetAllLLMTools 获取所有 LLM 工具定义
func (tc *MCPToolCaller) GetAllLLMTools(serviceToolsFilter map[string][]string) []*schema.ToolInfo {
	var llmTools []*schema.ToolInfo

	for serviceName, service := range tc.services {
		// 检查是否有工具过滤
		if serviceToolsFilter != nil {
			// 如果指定了该服务的工具列表
			if allowedTools, exists := serviceToolsFilter[serviceName]; exists {
				// 空数组表示不调用该服务的任何工具
				if len(allowedTools) == 0 {
					continue
				}
				// 只处理允许的工具
				for _, mcpTool := range service.Tools {
					// 检查工具是否在允许列表中
					found := false
					for _, allowedTool := range allowedTools {
						if allowedTool == mcpTool.Name {
							found = true
							break
						}
					}
					if !found {
						continue // 跳过不在允许列表中的工具
					}

					// 添加工具
					llmTools = append(llmTools, tc.convertMCPToolToLLMTool(serviceName, mcpTool))
				}
			}
			// 如果没有为该服务指定工具列表，则跳过该服务
			continue
		}

		// 没有过滤器，添加所有工具
		for _, mcpTool := range service.Tools {
			llmTools = append(llmTools, tc.convertMCPToolToLLMTool(serviceName, mcpTool))
		}
	}

	return llmTools
}

// convertMCPToolToLLMTool 将单个 MCP 工具转换为 LLM 工具
func (tc *MCPToolCaller) convertMCPToolToLLMTool(serviceName string, mcpTool client.MCPTool) *schema.ToolInfo {
	// 为工具名添加服务前缀，避免不同服务的工具名冲突
	toolName := fmt.Sprintf("%s__%s", serviceName, mcpTool.Name)

	// 将 MCP 的 InputSchema 转换为 schema.ToolInfo
	toolInfo := &schema.ToolInfo{
		Name: toolName,
		Desc: mcpTool.Description,
	}

	// 如果有 InputSchema，将其转换为 ParameterInfo map
	if mcpTool.InputSchema != nil && len(mcpTool.InputSchema) > 0 {
		params := make(map[string]*schema.ParameterInfo)

		// 从 InputSchema 中提取 properties
		if properties, ok := mcpTool.InputSchema["properties"].(map[string]interface{}); ok {
			for paramName, paramDefRaw := range properties {
				if paramDef, ok := paramDefRaw.(map[string]interface{}); ok {
					paramInfo := &schema.ParameterInfo{}

					// 设置类型
					if typeStr, ok := paramDef["type"].(string); ok {
						paramInfo.Type = typeStr
					}

					// 设置描述
					if desc, ok := paramDef["description"].(string); ok {
						paramInfo.Desc = desc
					}

					// 设置是否必需
					if required, ok := mcpTool.InputSchema["required"].([]interface{}); ok {
						for _, req := range required {
							if reqName, ok := req.(string); ok && reqName == paramName {
								paramInfo.Required = true
								break
							}
						}
					}

					params[paramName] = paramInfo
				}
			}
		}

		// 如果成功解析了参数，使用 NewParamsOneOfByParams
		if len(params) > 0 {
			toolInfo.ParamsOneOf = schema.NewParamsOneOfByParams(params)
		}
	}

	return toolInfo
}

// CallToolsWithLLM 使用 LLM 智能选择并调用工具
// serviceToolsFilter: 如果不为 nil，则只允许 LLM 调用指定服务的指定工具
// 返回值：documents（工具调用结果）, mcpResults（MCP结果）, finalAnswer（LLM最终答案）, error
func (tc *MCPToolCaller) CallToolsWithLLM(ctx context.Context, modelID string, question string, convID string, serviceToolsFilter map[string][]string) ([]*schema.Document, []*v1.MCPResult, string, error) {
	// 1. 准备工具列表（根据过滤器）
	llmTools := tc.GetAllLLMTools(serviceToolsFilter)
	if len(llmTools) == 0 {
		g.Log().Info(ctx, "没有可用的MCP工具")
		return nil, nil, "", nil
	}

	g.Log().Infof(ctx, "准备 %d 个 MCP 工具", len(llmTools))

	// 2. 构建初始消息
	systemPrompt := "你是一个智能助手，可以使用工具来帮助回答用户问题。\n" +
		"规则：\n" +
		"1. 根据用户问题判断是否需要使用工具\n" +
		"2. 如果需要工具，选择最合适的工具并提供正确的参数\n" +
		"3. 如果不需要工具，直接回答问题\n" +
		"4. 收到工具执行结果后，基于结果生成最终答案"

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: question,
		},
	}

	// 3. 调用 LLM（最多循环 5 次以支持多轮工具调用）
	chatInstance := chat.GetChat()
	maxIterations := 5
	var allDocuments []*schema.Document
	var allMCPResults []*v1.MCPResult
	var finalAnswer string                    // 保存 LLM 的最终文本回答
	var toolCallLogs []map[string]interface{} // 记录工具调用日志

	for iteration := 0; iteration < maxIterations; iteration++ {
		g.Log().Infof(ctx, "[MCP工具调用] 第 %d/%d 轮迭代开始，当前消息数: %d", iteration+1, maxIterations, len(messages))

		// 调用 LLM
		response, err := chatInstance.GenerateWithTools(ctx, modelID, messages, llmTools)
		if err != nil {
			g.Log().Errorf(ctx, "[MCP工具调用] 第 %d 轮LLM调用失败: %v", iteration+1, err)
			return nil, nil, "", errors.Newf(errors.ErrLLMCallFailed, "LLM 调用失败: %v", err)
		}

		g.Log().Infof(ctx, "[MCP工具调用] 第 %d 轮LLM响应 - Content长度: %d, ToolCalls数: %d",
			iteration+1, len(response.Content), len(response.ToolCalls))

		// 将 LLM 响应添加到消息历史
		messages = append(messages, response)

		// 4. 检查是否有工具调用
		if len(response.ToolCalls) == 0 {
			// 没有工具调用，LLM 已经给出最终答案
			finalAnswer = response.Content
			g.Log().Infof(ctx, "LLM 未调用任何工具，给出最终答案（长度: %d）", len(finalAnswer))
			break
		}

		// 5. 执行所有工具调用
		g.Log().Infof(ctx, "调用 %d 个工具", len(response.ToolCalls))

		for idx, toolCall := range response.ToolCalls {
			// 解析工具名
			serviceName, toolName := client.ParseToolName(toolCall.Function.Name)

			// 解析参数
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				errMsg := fmt.Sprintf("参数解析错误: %v", err)
				g.Log().Errorf(ctx, "[工具 %d/%d] %s", idx+1, len(response.ToolCalls), errMsg)

				// 添加错误响应到消息历史
				messages = append(messages, &schema.Message{
					Role:       schema.Tool,
					Content:    errMsg,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			// 调用工具
			result, mcpResult, err := tc.callSingleTool(ctx, serviceName, toolName, args, convID)
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

			// 收集结果
			allDocuments = append(allDocuments, result)
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

			// 将工具执行结果添加到消息历史，供 LLM 下次调用时使用
			toolResultMsg := &schema.Message{
				Role:       schema.Tool,
				Content:    mcpResult.Content,
				ToolCallID: toolCall.ID,
			}
			messages = append(messages, toolResultMsg)
		}

		// 如果这是最后一次迭代，需要再调用一次 LLM 让它基于工具结果给出最终答案
		if iteration == maxIterations-1 {
			g.Log().Warning(ctx, "达到最大工具调用迭代次数，尝试获取最终答案")

			// 最后一次调用 LLM（不提供工具，强制它给出最终答案）
			finalResponse, err := chatInstance.GenerateWithTools(ctx, modelID, messages, []*schema.ToolInfo{})
			if err != nil {
				g.Log().Errorf(ctx, "获取最终答案失败: %v", err)
			} else {
				finalAnswer = finalResponse.Content
			}
			break
		}
	}

	// 6. 记录工具调用日志（仅用于内部统计）
	_ = toolCallLogs

	// 7. 记录完成日志
	if len(allMCPResults) > 0 {
		g.Log().Infof(ctx, "MCP 工具调用完成: %d 个工具调用", len(allMCPResults))
	} else {
		g.Log().Infof(ctx, "MCP 流程完成: LLM 未调用工具")
	}

	return allDocuments, allMCPResults, finalAnswer, nil
}

// callSingleTool 调用单个工具
func (tc *MCPToolCaller) callSingleTool(
	ctx context.Context,
	serviceName string,
	toolName string,
	arguments map[string]interface{},
	convID string,
) (*schema.Document, *v1.MCPResult, error) {
	// 查找服务
	service, exists := tc.services[serviceName]
	if !exists {
		return nil, nil, errors.Newf(errors.ErrMCPServerNotFound, "服务 %s 不存在", serviceName)
	}

	startTime := time.Now()

	// 调用工具
	result, err := service.Client.CallTool(ctx, toolName, arguments)

	// 计算耗时
	duration := int(time.Since(startTime).Milliseconds())

	// 序列化请求和响应
	reqPayload, _ := json.Marshal(arguments)
	respPayload, _ := json.Marshal(result)

	// 记录调用日志
	logStatus := int8(1) // 成功
	errorMsg := ""
	if err != nil {
		logStatus = 0 // 失败
		errorMsg = err.Error()
	}

	logID := strings.ReplaceAll(uuid.New().String(), "-", "")
	callLog := &gormModel.MCPCallLog{
		ID:              logID,
		ConversationID:  convID,
		MCPRegistryID:   service.Registry.ID,
		MCPServiceName:  service.Registry.Name,
		ToolName:        toolName,
		RequestPayload:  string(reqPayload),
		ResponsePayload: string(respPayload),
		Status:          logStatus,
		ErrorMessage:    errorMsg,
		Duration:        duration,
	}

	// 使用缓存层保存MCP调用日志
	mcpLogCache := internalCache.GetMCPCallLogCache()
	if mcpLogCache != nil {
		// 使用缓存层（异步刷盘到数据库）
		if logErr := mcpLogCache.SaveMCPCallLog(ctx, callLog); logErr != nil {
			g.Log().Errorf(ctx, "保存 MCP 调用日志到缓存失败: %v", logErr)
		}
	} else {
		// 缓存层不可用，直接写数据库
		if logErr := dao.MCPCallLog.Create(ctx, callLog); logErr != nil {
			g.Log().Errorf(ctx, "创建 MCP 调用日志失败: %v", logErr)
		}
	}

	if err != nil {
		return nil, nil, err
	}

	// 提取文本内容
	var content string
	for _, c := range result.Content {
		if c.Type == "text" && c.Text != "" {
			content += c.Text + "\n"
		}
	}
	content = strings.TrimSpace(content)

	// 构建文档
	doc := &schema.Document{
		ID:      logID,
		Content: content,
		MetaData: map[string]interface{}{
			"source":    "mcp",
			"service":   serviceName,
			"tool":      toolName,
			"tool_desc": "", // 可以从 service.Tools 中查找
		},
	}

	// 查找工具描述
	for _, tool := range service.Tools {
		if tool.Name == toolName {
			doc.MetaData["tool_desc"] = tool.Description
			break
		}
	}

	// 构建 MCP 结果
	mcpResult := &v1.MCPResult{
		ServiceName: serviceName,
		ToolName:    toolName,
		Content:     content,
	}

	return doc, mcpResult, nil
}

// Close 关闭所有 MCP 客户端连接
func (tc *MCPToolCaller) Close() {
	for _, service := range tc.services {
		if err := service.Client.Close(); err != nil {
			g.Log().Errorf(context.Background(), "关闭 MCP 客户端失败: %v", err)
		}
	}
}
