package agent_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools/knowledge_retrieval"
	"github.com/Malowking/kbgo/core/agent_tools/mcp"
	"github.com/Malowking/kbgo/core/agent_tools/mcp/client"
	"github.com/Malowking/kbgo/core/agent_tools/nl2sql"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/logic/chat"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// ToolExecutor 统一的工具执行器
type ToolExecutor struct{}

// NewToolExecutor 创建工具执行器
func NewToolExecutor() *ToolExecutor {
	return &ToolExecutor{}
}

// ExecuteResult 工具执行结果
type ExecuteResult struct {
	Documents    []*schema.Document   // 工具返回的文档
	NL2SQLResult *v1.NL2SQLChatResult // NL2SQL结果
	MCPResults   []*v1.MCPResult      // MCP结果
	FinalAnswer  string               // 最终答案(如果工具直接返回)
	Error        string               // 错误信息
}

// ToolCallResult 单个工具调用结果
type ToolCallResult struct {
	Content      string               // 工具返回的文本内容
	Documents    []*schema.Document   // 工具返回的文档
	NL2SQLResult *v1.NL2SQLChatResult // NL2SQL结果
	MCPResults   []*v1.MCPResult      // MCP结果
	Error        error                // 错误信息
}

// Execute 执行所有配置的工具（统一由 LLM 选择）
func (e *ToolExecutor) Execute(ctx context.Context, tools []*v1.ToolConfig, question string, modelID string, embeddingModelID string, documents []*schema.Document, systemPrompt string, convID string, messageID string) (*ExecuteResult, error) {
	result := &ExecuteResult{
		Documents: make([]*schema.Document, 0),
	}

	// 如果没有配置工具,直接返回
	if tools == nil || len(tools) == 0 {
		g.Log().Infof(ctx, "No tools configured")
		return result, nil
	}

	// 1. 收集所有可用的工具定义
	var allLLMTools []*schema.ToolInfo
	var localToolsConfig *v1.ToolConfig
	var mcpServiceTools map[string][]string

	for _, toolConfig := range tools {
		if !toolConfig.Enabled {
			g.Log().Infof(ctx, "Tool type '%s' is disabled, skipping", toolConfig.Type)
			continue
		}

		switch toolConfig.Type {
		case "local_tools":
			localToolsConfig = toolConfig
			// 获取本地工具的 LLM 定义
			localTools := e.GetLocalToolDefinitions(toolConfig)
			allLLMTools = append(allLLMTools, localTools...)
			g.Log().Infof(ctx, "Added %d local tools", len(localTools))

		case "mcp":
			// 提取 MCP 工具配置
			if serviceTools, ok := toolConfig.Config["service_tools"].(map[string]interface{}); ok {
				mcpServiceTools = convertToServiceToolsMap(serviceTools)
			}
		}
	}

	// 2. 如果有 MCP 工具，获取 MCP 工具定义
	var mcpToolCaller *mcp.MCPToolCaller
	if mcpServiceTools != nil && len(mcpServiceTools) > 0 {
		var err error
		mcpToolCaller, err = mcp.NewMCPToolCaller(ctx)
		if err != nil {
			g.Log().Errorf(ctx, "Failed to create MCP tool caller: %v", err)
		} else {
			defer mcpToolCaller.Close()
			mcpTools := mcpToolCaller.GetAllLLMTools(mcpServiceTools)
			allLLMTools = append(allLLMTools, mcpTools...)
			g.Log().Infof(ctx, "Added %d MCP tools", len(mcpTools))
		}
	}

	// 3. 如果没有可用工具，直接返回
	if len(allLLMTools) == 0 {
		g.Log().Info(ctx, "No tools available")
		return result, nil
	}

	g.Log().Infof(ctx, "Total %d tools available for LLM selection", len(allLLMTools))

	// 4. 构建增强的系统提示词（结合 Agent 的 SystemPrompt 和工具优先级）
	enhancedSystemPrompt := e.buildEnhancedSystemPrompt(systemPrompt, tools)

	// 5. 调用 LLM 智能选择并执行工具
	return e.executeWithLLM(ctx, question, modelID, embeddingModelID, documents,
		enhancedSystemPrompt, allLLMTools, localToolsConfig, mcpToolCaller, convID, messageID)
}

// buildFullQuestion 构建包含知识检索结果的完整问题
func (e *ToolExecutor) buildFullQuestion(question string, documents []*schema.Document) string {
	var builder strings.Builder

	builder.WriteString(question)

	// 如果有知识库检索内容，添加到问题中
	if len(documents) > 0 {
		builder.WriteString("\n\n【知识库检索结果】\n")
		for i, doc := range documents {
			builder.WriteString(fmt.Sprintf("文档%d: %s\n", i+1, doc.Content))
		}
	}

	return builder.String()
}

// GetLocalToolDefinitions 获取本地工具的 LLM 定义
func (e *ToolExecutor) GetLocalToolDefinitions(config *v1.ToolConfig) []*schema.ToolInfo {
	var tools []*schema.ToolInfo

	if config.Config == nil {
		return tools
	}

	// 知识检索工具
	if krConfig, ok := config.Config["knowledge_retrieval"].(map[string]interface{}); ok {
		if knowledgeID, _ := krConfig["knowledge_id"].(string); knowledgeID != "" {
			tools = append(tools, &schema.ToolInfo{
				Name: "knowledge_retrieval",
				Desc: "从指定的知识库中检索相关文档。当用户问题需要查询特定领域知识、文档内容或历史信息时使用此工具。适用场景：查询产品文档、技术规范、历史记录、政策文件等。",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"query": {
						Type:     "string",
						Desc:     "检索查询语句，应该是用户问题的关键词或改写后的查询",
						Required: true,
					},
				}),
			})
		}
	}

	// NL2SQL 工具
	if nl2sqlConfig, ok := config.Config["nl2sql"].(map[string]interface{}); ok {
		if datasource, _ := nl2sqlConfig["datasource"].(string); datasource != "" {
			tools = append(tools, &schema.ToolInfo{
				Name: "nl2sql",
				Desc: "将自然语言问题转换为SQL查询并执行，返回数据库查询结果。当用户问题涉及数据统计、查询、分析时使用此工具。适用场景：查询销售数据、统计用户数量、分析趋势、生成报表等。",
				ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
					"question": {
						Type:     "string",
						Desc:     "需要转换为SQL的自然语言问题",
						Required: true,
					},
				}),
			})
		}
	}

	// 文件导出工具
	if _, ok := config.Config["file_export"].(map[string]interface{}); ok {
		tools = append(tools, &schema.ToolInfo{
			Name: "file_export",
			Desc: "将数据导出为文件（Excel、CSV等格式）。当用户明确要求导出数据、下载文件或生成报表文件时使用此工具。",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"data": {
					Type:     "array",
					Desc:     "要导出的数据，JSON数组格式",
					Required: true,
				},
				"format": {
					Type:     "string",
					Desc:     "导出格式：excel 或 csv，默认为 excel",
					Required: false,
				},
			}),
		})
	}

	return tools
}

// buildEnhancedSystemPrompt 构建增强的系统提示词
func (e *ToolExecutor) buildEnhancedSystemPrompt(agentSystemPrompt string, tools []*v1.ToolConfig) string {
	basePrompt := `你是一个智能助手，可以使用工具来帮助回答用户问题。

## 工具使用规则：
1. **仔细分析用户问题**，判断是否需要使用工具
2. **选择最合适的工具**：
   - 需要查询知识库文档时 → 使用 knowledge_retrieval
   - 需要查询/统计数据时 → 使用 nl2sql
   - 需要调用外部服务时 → 使用相应的 MCP 工具
   - 需要导出数据时 → 使用 file_export
3. **可以组合使用多个工具**，例如先用 nl2sql 查询数据，再用 file_export 导出
4. **如果不需要工具**，直接基于已有信息回答
5. **收到工具结果后**，基于结果生成清晰、准确的最终答案

`

	// 添加工具优先级建议
	toolOrderSuggestion := e.buildToolOrderSuggestion(tools)
	if toolOrderSuggestion != "" {
		basePrompt += toolOrderSuggestion + "\n"
	}

	// 如果有 Agent 的自定义提示词，追加到后面
	if agentSystemPrompt != "" {
		basePrompt += "\n## Agent 角色定位：\n" + agentSystemPrompt
	}

	return basePrompt
}

// buildToolOrderSuggestion 根据工具配置的 Priority 构建工具使用顺序建议
func (e *ToolExecutor) buildToolOrderSuggestion(tools []*v1.ToolConfig) string {
	if tools == nil || len(tools) == 0 {
		return ""
	}

	// 收集所有有优先级的工具
	type toolWithPriority struct {
		name     string
		priority int
		config   *v1.ToolConfig
	}
	var prioritizedTools []toolWithPriority

	for _, toolConfig := range tools {
		if !toolConfig.Enabled || toolConfig.Priority == nil {
			continue
		}

		// 提取工具名称
		if toolConfig.Type == "local_tools" && toolConfig.Config != nil {
			// 本地工具：knowledge_retrieval, nl2sql, file_export
			for toolName := range toolConfig.Config {
				prioritizedTools = append(prioritizedTools, toolWithPriority{
					name:     toolName,
					priority: *toolConfig.Priority,
					config:   toolConfig,
				})
			}
		} else if toolConfig.Type == "mcp" {
			// MCP 工具
			prioritizedTools = append(prioritizedTools, toolWithPriority{
				name:     "mcp_tools",
				priority: *toolConfig.Priority,
				config:   toolConfig,
			})
		}
	}

	// 如果没有设置优先级的工具，不生成建议
	if len(prioritizedTools) == 0 {
		return ""
	}

	// 按优先级排序（数字越小优先级越高）
	for i := 0; i < len(prioritizedTools); i++ {
		for j := i + 1; j < len(prioritizedTools); j++ {
			if prioritizedTools[i].priority > prioritizedTools[j].priority {
				prioritizedTools[i], prioritizedTools[j] = prioritizedTools[j], prioritizedTools[i]
			}
		}
	}

	// 构建建议文本
	var builder strings.Builder
	builder.WriteString("## 工具使用顺序建议：\n")
	builder.WriteString("根据配置，建议按以下优先级使用工具（这只是建议，你可以根据实际情况灵活调整）：\n\n")

	toolNameMap := map[string]string{
		"knowledge_retrieval": "knowledge_retrieval（知识库检索）",
		"nl2sql":              "nl2sql（数据查询）",
		"file_export":         "file_export（文件导出）",
		"mcp_tools":           "MCP工具（外部服务调用）",
	}

	for i, tool := range prioritizedTools {
		displayName := toolNameMap[tool.name]
		if displayName == "" {
			displayName = tool.name
		}
		builder.WriteString(fmt.Sprintf("%d. **优先级 %d**：%s\n", i+1, tool.priority, displayName))
	}

	builder.WriteString("\n**注意**：这个顺序只是建议，你应该根据用户的具体问题灵活选择工具。例如：\n")
	builder.WriteString("- 如果用户明确要求查询数据，即使 knowledge_retrieval 优先级更高，也应该直接使用 nl2sql\n")
	builder.WriteString("- 如果某个工具的结果依赖于另一个工具，应该先执行被依赖的工具\n")

	return builder.String()
}

// dispatchToolCall 分发工具调用到具体的执行器
func (e *ToolExecutor) dispatchToolCall(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
	localToolsConfig *v1.ToolConfig,
	mcpToolCaller *mcp.MCPToolCaller,
	question string,
	modelID string,
	embeddingModelID string,
	convID string,
) (*ToolCallResult, error) {

	// 判断是本地工具还是 MCP 工具
	if strings.Contains(toolName, "__") {
		// MCP 工具（格式：service__tool）
		return e.executeMCPTool(ctx, toolName, args, mcpToolCaller, convID)
	}

	// 本地工具
	switch toolName {
	case "knowledge_retrieval":
		return e.executeKnowledgeRetrieval(ctx, args, localToolsConfig, question)

	case "nl2sql":
		return e.executeNL2SQL(ctx, args, localToolsConfig, modelID, embeddingModelID)

	case "file_export":
		return e.executeFileExport(ctx, args, localToolsConfig)

	default:
		return nil, fmt.Errorf("未知工具: %s", toolName)
	}
}

// executeKnowledgeRetrieval 执行知识检索工具
func (e *ToolExecutor) executeKnowledgeRetrieval(
	ctx context.Context,
	args map[string]interface{},
	config *v1.ToolConfig,
	originalQuestion string,
) (*ToolCallResult, error) {

	// 从参数中提取查询语句
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("knowledge_retrieval: 缺少必需参数 'query'")
	}

	g.Log().Infof(ctx, "[知识检索工具] 执行查询: %s", query)

	// 从配置中解析知识检索配置
	krConfig, ok := config.Config["knowledge_retrieval"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("knowledge_retrieval: 配置不存在")
	}

	krToolConfig := knowledge_retrieval.ParseConfig(krConfig)

	// 执行知识检索
	krTool := knowledge_retrieval.NewKnowledgeRetrievalTool()
	krResult, err := krTool.Execute(ctx, krToolConfig, query)
	if err != nil {
		return nil, fmt.Errorf("知识检索失败: %w", err)
	}

	// 构建返回内容
	var contentBuilder strings.Builder
	if len(krResult.Documents) > 0 {
		contentBuilder.WriteString(fmt.Sprintf("检索到 %d 个相关文档：\n\n", len(krResult.Documents)))
		for i, doc := range krResult.Documents {
			contentBuilder.WriteString(fmt.Sprintf("文档 %d:\n%s\n\n", i+1, doc.Content))
		}
	} else {
		contentBuilder.WriteString("未检索到相关文档")
	}

	return &ToolCallResult{
		Content:   contentBuilder.String(),
		Documents: krResult.Documents,
	}, nil
}

// executeNL2SQL 执行 NL2SQL 工具
func (e *ToolExecutor) executeNL2SQL(
	ctx context.Context,
	args map[string]interface{},
	config *v1.ToolConfig,
	modelID string,
	embeddingModelID string,
) (*ToolCallResult, error) {

	// 从参数中提取问题
	question, ok := args["question"].(string)
	if !ok || question == "" {
		return nil, fmt.Errorf("nl2sql: 缺少必需参数 'question'")
	}

	g.Log().Infof(ctx, "[NL2SQL工具] 执行查询: %s", question)

	// 从配置中获取数据源ID
	nl2sqlConfig, ok := config.Config["nl2sql"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("nl2sql: 配置不存在")
	}

	datasource, ok := nl2sqlConfig["datasource"].(string)
	if !ok || datasource == "" {
		return nil, fmt.Errorf("nl2sql: 缺少数据源配置")
	}

	// 执行 NL2SQL
	nl2sqlTool := nl2sql.NewNL2SQLTool()
	nl2sqlResult, err := nl2sqlTool.DetectAndExecute(ctx, question, datasource, modelID, embeddingModelID)
	if err != nil {
		return nil, fmt.Errorf("NL2SQL执行失败: %w", err)
	}

	// 构建返回内容
	var contentBuilder strings.Builder
	if nl2sqlResult.IsNL2SQLQuery {
		if nl2sqlResult.Error == "" {
			// 成功执行
			contentBuilder.WriteString(fmt.Sprintf("SQL查询成功执行\n\n"))
			contentBuilder.WriteString(fmt.Sprintf("生成的SQL: %s\n\n", nl2sqlResult.SQL))
			contentBuilder.WriteString(fmt.Sprintf("返回 %d 行数据", nl2sqlResult.RowCount))
			if nl2sqlResult.DataTruncated {
				contentBuilder.WriteString(fmt.Sprintf("（总共 %d 行，已截断）", nl2sqlResult.TotalRowCount))
			}
			contentBuilder.WriteString("\n\n")

			// 添加数据摘要
			if len(nl2sqlResult.Data) > 0 {
				dataJSON, _ := json.MarshalIndent(nl2sqlResult.Data, "", "  ")
				contentBuilder.WriteString(fmt.Sprintf("数据结果:\n%s", string(dataJSON)))
			}
		} else {
			// 执行失败
			contentBuilder.WriteString(fmt.Sprintf("SQL查询失败\n\n"))
			contentBuilder.WriteString(fmt.Sprintf("生成的SQL: %s\n\n", nl2sqlResult.SQL))
			contentBuilder.WriteString(fmt.Sprintf("错误信息: %s", nl2sqlResult.Error))
		}
	} else {
		contentBuilder.WriteString("该问题不是数据查询类问题，无需使用NL2SQL工具")
	}

	// 构建 NL2SQLResult
	var nl2sqlChatResult *v1.NL2SQLChatResult
	if nl2sqlResult.IsNL2SQLQuery {
		nl2sqlChatResult = &v1.NL2SQLChatResult{
			QueryLogID:      nl2sqlResult.QueryLogID,
			SQL:             nl2sqlResult.SQL,
			Columns:         nl2sqlResult.Columns,
			Data:            nl2sqlResult.Data,
			RowCount:        nl2sqlResult.RowCount,
			TotalRowCount:   nl2sqlResult.TotalRowCount,
			Explanation:     nl2sqlResult.Explanation,
			IntentType:      nl2sqlResult.IntentType,
			NeedLLMAnalysis: nl2sqlResult.NeedLLMAnalysis,
			AnalysisFocus:   nl2sqlResult.AnalysisFocus,
			DataTruncated:   nl2sqlResult.DataTruncated,
			FileURL:         nl2sqlResult.FileURL,
			Error:           nl2sqlResult.Error,
		}
	}

	return &ToolCallResult{
		Content:      contentBuilder.String(),
		Documents:    nl2sqlResult.Documents,
		NL2SQLResult: nl2sqlChatResult,
	}, nil
}

// executeFileExport 执行文件导出工具
func (e *ToolExecutor) executeFileExport(
	ctx context.Context,
	args map[string]interface{},
	config *v1.ToolConfig,
) (*ToolCallResult, error) {

	// TODO: 实现文件导出逻辑
	g.Log().Warningf(ctx, "[文件导出工具] 功能尚未实现")

	return &ToolCallResult{
		Content: "文件导出功能尚未实现",
	}, nil
}

// executeMCPTool 执行 MCP 工具
func (e *ToolExecutor) executeMCPTool(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
	mcpToolCaller *mcp.MCPToolCaller,
	convID string,
) (*ToolCallResult, error) {

	// 解析工具名
	serviceName, actualToolName := client.ParseToolName(toolName)

	g.Log().Infof(ctx, "[MCP工具] 调用 %s.%s", serviceName, actualToolName)

	// 调用工具
	doc, mcpResult, err := mcpToolCaller.CallSingleTool(ctx, serviceName, actualToolName, args, convID)
	if err != nil {
		return nil, fmt.Errorf("MCP工具调用失败: %w", err)
	}

	return &ToolCallResult{
		Content:    mcpResult.Content,
		Documents:  []*schema.Document{doc},
		MCPResults: []*v1.MCPResult{mcpResult},
	}, nil
}

// convertToServiceToolsMap 转换 service_tools 配置为 map[string][]string
func convertToServiceToolsMap(serviceTools map[string]interface{}) map[string][]string {
	result := make(map[string][]string)
	for service, tools := range serviceTools {
		if toolList, ok := tools.([]interface{}); ok {
			strTools := make([]string, 0, len(toolList))
			for _, tool := range toolList {
				if toolStr, ok := tool.(string); ok {
					strTools = append(strTools, toolStr)
				}
			}
			result[service] = strTools
		}
	}
	return result
}

// executeWithLLM 使用 LLM 智能选择并执行工具
func (e *ToolExecutor) executeWithLLM(
	ctx context.Context,
	question string,
	modelID string,
	embeddingModelID string,
	documents []*schema.Document,
	systemPrompt string,
	allTools []*schema.ToolInfo,
	localToolsConfig *v1.ToolConfig,
	mcpToolCaller *mcp.MCPToolCaller,
	convID string,
	messageID string,
) (*ExecuteResult, error) {

	result := &ExecuteResult{
		Documents: make([]*schema.Document, 0),
	}

	// 获取 HTTP Response 对象（用于发送 SSE 事件）
	var httpResp *ghttp.Response
	httpReq := ghttp.RequestFromCtx(ctx)
	if httpReq != nil {
		httpResp = httpReq.Response
	}

	// 1. 构建初始消息
	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: systemPrompt,
		},
		{
			Role:    schema.User,
			Content: e.buildFullQuestion(question, documents),
		},
	}

	// 2. 多轮工具调用（最多 5 轮）
	chatInstance := chat.GetChat()
	maxIterations := 5

	for iteration := 0; iteration < maxIterations; iteration++ {
		g.Log().Infof(ctx, "[统一工具调用] 第 %d/%d 轮", iteration+1, maxIterations)

		// 发送 LLM 迭代事件
		if httpResp != nil {
			common.WriteLLMIteration(httpResp, messageID, iteration+1, maxIterations, fmt.Sprintf("正在进行第 %d 轮工具选择...", iteration+1))
		}

		// 调用 LLM
		response, err := chatInstance.GenerateWithTools(ctx, modelID, messages, allTools)
		if err != nil {
			return nil, fmt.Errorf("LLM调用失败: %w", err)
		}

		messages = append(messages, response)

		// 如果没有工具调用，结束循环
		if len(response.ToolCalls) == 0 {
			g.Log().Info(ctx, "LLM 未调用工具，工具执行完成")
			// 如果 LLM 返回了文本内容，设置为最终答案
			if response.Content != "" {
				result.FinalAnswer = response.Content
			}
			break
		}

		// 3. 执行所有工具调用
		g.Log().Infof(ctx, "执行 %d 个工具调用", len(response.ToolCalls))

		for _, toolCall := range response.ToolCalls {
			toolName := toolCall.Function.Name

			// 解析参数
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				errMsg := fmt.Sprintf("参数解析失败: %v", err)
				g.Log().Errorf(ctx, "[工具 %s] %s", toolName, errMsg)

				// 添加错误响应
				messages = append(messages, &schema.Message{
					Role:       schema.Tool,
					Content:    errMsg,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			// 发送工具调用开始事件
			if httpResp != nil {
				common.WriteToolCallStart(httpResp, messageID, toolCall.ID, toolName, args)
			}

			// 记录开始时间
			startTime := time.Now()

			// 根据工具名称分发执行
			toolResult, err := e.dispatchToolCall(ctx, toolName, args,
				localToolsConfig, mcpToolCaller, question, modelID, embeddingModelID, convID)

			// 计算执行时间
			duration := time.Since(startTime).Milliseconds()

			// 发送工具调用结束事件
			if httpResp != nil {
				var resultSummary string
				if err != nil {
					resultSummary = fmt.Sprintf("工具执行失败: %v", err)
				} else if toolResult != nil {
					resultSummary = toolResult.Content
					// 限制结果长度
					if len(resultSummary) > 200 {
						resultSummary = resultSummary[:200] + "..."
					}
				}
				common.WriteToolCallEnd(httpResp, messageID, toolCall.ID, toolName, resultSummary, err, duration)
			}

			if err != nil {
				errMsg := fmt.Sprintf("工具执行失败: %v", err)
				g.Log().Errorf(ctx, "[工具 %s] %s", toolName, errMsg)

				// 添加错误响应
				messages = append(messages, &schema.Message{
					Role:       schema.Tool,
					Content:    errMsg,
					ToolCallID: toolCall.ID,
				})
				continue
			}

			// 收集结果
			if toolResult.Documents != nil {
				result.Documents = append(result.Documents, toolResult.Documents...)
			}
			if toolResult.NL2SQLResult != nil {
				result.NL2SQLResult = toolResult.NL2SQLResult
			}
			if toolResult.MCPResults != nil {
				result.MCPResults = append(result.MCPResults, toolResult.MCPResults...)
			}

			// 添加工具结果到消息历史
			messages = append(messages, &schema.Message{
				Role:       schema.Tool,
				Content:    toolResult.Content,
				ToolCallID: toolCall.ID,
			})
		}

		// 如果是最后一轮，强制 LLM 给出最终答案
		if iteration == maxIterations-1 {
			g.Log().Warning(ctx, "达到最大迭代次数，获取最终答案")

			// 最后一次调用 LLM（不提供工具）
			finalResponse, err := chatInstance.GenerateWithTools(ctx, modelID, messages, []*schema.ToolInfo{})
			if err != nil {
				g.Log().Errorf(ctx, "获取最终答案失败: %v", err)
			} else {
				result.FinalAnswer = finalResponse.Content
			}
			break
		}
	}

	return result, nil
}
