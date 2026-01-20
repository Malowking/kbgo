package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/os/gctx"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools/framework"
	"github.com/Malowking/kbgo/core/agent_tools/registry"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/schema"
	internalCache "github.com/Malowking/kbgo/internal/cache"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

// MCPServiceClient MCP 服务客户端封装
type MCPServiceClient struct {
	Registry *gormModel.MCPRegistry
	Client   *MCPClient
	Tools    []MCPTool
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

		mcpClient := NewMCPClient(registry)

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
		var tools []MCPTool

		// 首先尝试从数据库缓存中获取
		if registry.Tools != "" && registry.Tools != "[]" {
			var toolInfos []v1.MCPToolInfo
			if err := json.Unmarshal([]byte(registry.Tools), &toolInfos); err == nil {
				tools = make([]MCPTool, len(toolInfos))
				for i, info := range toolInfos {
					tools[i] = MCPTool{
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
func (tc *MCPToolCaller) convertMCPToolToLLMTool(serviceName string, mcpTool MCPTool) *schema.ToolInfo {
	// 为工具名添加 MCP 前缀，避免不同服务的工具名冲突
	toolName := fmt.Sprintf("mcp_%s_%s", serviceName, mcpTool.Name)

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

// CallSingleTool 调用单个工具（导出方法供外部使用）
func (tc *MCPToolCaller) CallSingleTool(
	ctx context.Context,
	serviceName string,
	toolName string,
	arguments map[string]interface{},
	convID string,
) (*schema.Document, *v1.MCPResult, error) {
	return tc.callSingleTool(ctx, serviceName, toolName, arguments, convID)
}

// callSingleTool 调用单个工具（内部方法）
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

// Execute 实现 ToolExecutor 接口，执行 MCP 工具
func (tc *MCPToolCaller) Execute(
	ctx framework.ToolContext,
	tool framework.ToolDefinition,
	input map[string]interface{},
) (*schema.ToolResult, error) {
	// 解析工具名（格式：mcp_service_tool）
	serviceName, toolName := parseMCPToolName(tool.Name)
	if serviceName == "" || toolName == "" {
		return schema.NewToolResultFromError(
			schema.NewValidationError(
				"无效的MCP工具名称格式",
				fmt.Sprintf("期望格式: mcp_service_tool, 实际: %s", tool.Name),
			),
		), nil
	}

	g.Log().Infof(ctx.Context, "[MCPToolCaller] 执行工具 %s.%s", serviceName, toolName)

	// 调用 MCP 工具
	doc, _, err := tc.CallSingleTool(ctx.Context, serviceName, toolName, input, ctx.SessionID)
	if err != nil {
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("MCP工具调用失败: %v", err), ""),
		), nil
	}

	// 转换为 ToolResult
	toolResult := schema.NewToolResult(doc.Content)

	// 添加元数据
	if doc.MetaData != nil {
		for key, value := range doc.MetaData {
			toolResult.WithMetadata(key, value)
		}
	}

	return toolResult, nil
}

// parseMCPToolName 解析MCP工具名（格式：mcp_service_tool）
func parseMCPToolName(fullName string) (serviceName, toolName string) {
	if !strings.HasPrefix(fullName, "mcp_") {
		return "", ""
	}

	trimmed := strings.TrimPrefix(fullName, "mcp_")

	// 使用单下划线分隔
	parts := strings.SplitN(trimmed, "_", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}

	return "", ""
}

// GetAllClients 获取所有 MCP 客户端（用于工具注册）
func (tc *MCPToolCaller) GetAllClients() map[string]*MCPClient {
	clients := make(map[string]*MCPClient)
	for serviceName, service := range tc.services {
		clients[serviceName] = service.Client
	}
	return clients
}

// GetAllServices 获取所有 MCP 服务客户端（包含工具列表）
func (tc *MCPToolCaller) GetAllServices() map[string]*MCPServiceClient {
	return tc.services
}

// GetFilteredLLMToolsAndRegister 获取过滤后的 LLM 工具并同时注册到 registry
// 这个方法复用了 GetAllLLMTools 的逻辑，同时完成注册，避免重复遍历
func (tc *MCPToolCaller) GetFilteredLLMToolsAndRegister(
	ctx context.Context,
	serviceToolsFilter map[string][]string,
	toolRegistry registry.ToolRegistry,
) ([]*schema.ToolInfo, error) {
	var llmTools []*schema.ToolInfo
	registeredServices := make(map[string]bool) // 记录已注册的服务，避免重复注册

	for serviceName, service := range tc.services {
		// 应用工具过滤逻辑
		var toolsToProcess []MCPTool

		if serviceToolsFilter != nil {
			if allowedTools, exists := serviceToolsFilter[serviceName]; exists {
				if len(allowedTools) == 0 {
					continue
				}
				// 只处理允许的工具
				for _, mcpTool := range service.Tools {
					found := false
					for _, allowedTool := range allowedTools {
						if allowedTool == mcpTool.Name {
							found = true
							break
						}
					}
					if found {
						toolsToProcess = append(toolsToProcess, mcpTool)
					}
				}
			}
			// 如果没有为该服务指定工具列表，则跳过该服务
			continue
		} else {
			// 没有过滤器，处理所有工具
			toolsToProcess = service.Tools
		}

		// 如果有工具需要处理
		if len(toolsToProcess) > 0 {
			// 转换为 LLM 工具
			for _, mcpTool := range toolsToProcess {
				llmTool := tc.convertMCPToolToLLMTool(serviceName, mcpTool)
				llmTools = append(llmTools, llmTool)
			}

			// 如果提供了 registry 且该服务还未注册，注册该服务的所有工具（只注册一次）
			if toolRegistry != nil && !registeredServices[serviceName] {
				if err := RegisterMCPTools(ctx, toolRegistry, service.Client, serviceName); err != nil {
					g.Log().Warningf(ctx, "[MCPToolCaller] Failed to register tools from %s: %v", serviceName, err)
				} else {
					registeredServices[serviceName] = true
					g.Log().Infof(ctx, "[MCPToolCaller] Registered tools from service: %s", serviceName)
				}
			}
		}
	}

	return llmTools, nil
}

// Close 关闭所有 MCP 客户端连接
func (tc *MCPToolCaller) Close() {
	for _, service := range tc.services {
		if err := service.Client.Close(); err != nil {
			g.Log().Errorf(gctx.New(), "关闭 MCP 客户端失败: %v", err)
		}
	}
}
