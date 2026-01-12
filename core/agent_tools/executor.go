package agent_tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools/claude_skills"
	"github.com/Malowking/kbgo/core/agent_tools/file_export"
	"github.com/Malowking/kbgo/core/agent_tools/knowledge_retrieval"
	"github.com/Malowking/kbgo/core/agent_tools/mcp"
	"github.com/Malowking/kbgo/core/agent_tools/mcp/client"
	"github.com/Malowking/kbgo/core/agent_tools/nl2sql"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/history"
	"github.com/Malowking/kbgo/internal/logic/chat"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// ToolExecutor 统一的工具执行器
type ToolExecutor struct {
	skillManager *claude_skills.SkillManager // Skill 管理器
}

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
	FileURL      string               // 文件下载URL（用于文件导出等工具）
	ToolType     string               // 工具类型: local, mcp, skill
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
	var skillManager *claude_skills.SkillManager

	// TODO: 从上下文中获取用户ID
	ownerID := "default_user"

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

		case "claude_skills":
			// 加载用户的 Claude Skills
			skills, err := dao.ClaudeSkill.ListActive(ctx, ownerID)
			if err != nil {
				g.Log().Errorf(ctx, "Failed to load Claude Skills: %v", err)
			} else if len(skills) > 0 {
				// 从配置文件读取路径（不允许用户覆盖）
				venvBaseDir := g.Cfg().MustGet(ctx, "skills.venvBaseDir", "/data/kbgo_venvs").String()
				skillsDir := g.Cfg().MustGet(ctx, "skills.scriptsDir", "/data/kbgo_skills").String()

				g.Log().Infof(ctx, "Claude Skills config: venvBaseDir=%s, scriptsDir=%s", venvBaseDir, skillsDir)

				skillManager, err = claude_skills.NewSkillManager(venvBaseDir, skillsDir)
				if err != nil {
					g.Log().Errorf(ctx, "Failed to create Skill manager: %v", err)
				} else {
					// 注册所有启用的 Skills
					for _, skillModel := range skills {
						skill := convertGormSkillToExecutorSkill(skillModel)
						if err := skillManager.RegisterSkill(skill); err != nil {
							g.Log().Errorf(ctx, "Failed to register skill %s: %v", skill.Name, err)
						}
					}

					// 获取 LLM 工具定义
					skillTools := skillManager.GetLLMTools()
					allLLMTools = append(allLLMTools, skillTools...)
					g.Log().Infof(ctx, "Added %d Claude Skills", len(skillTools))

					// 保存 skillManager 到 executor
					e.skillManager = skillManager
				}
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
   - 需要执行自定义脚本或特殊功能时 → 使用 Claude Skills（以 skill__ 开头的工具）
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
		} else if toolConfig.Type == "claude_skills" {
			// Claude Skills
			prioritizedTools = append(prioritizedTools, toolWithPriority{
				name:     "claude_skills",
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
		"claude_skills":       "Claude Skills（自定义脚本）",
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

	// 判断工具类型
	if strings.HasPrefix(toolName, "skill__") {
		// Claude Skill 工具（格式：skill__tool_name）
		return e.executeSkillTool(ctx, toolName, args)
	}

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
		ToolType:  "local",
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
		ToolType:     "local",
	}, nil
}

// executeFileExport 执行文件导出工具
func (e *ToolExecutor) executeFileExport(
	ctx context.Context,
	args map[string]interface{},
	config *v1.ToolConfig,
) (*ToolCallResult, error) {

	g.Log().Infof(ctx, "[文件导出工具] 开始执行")

	// 1. 从参数中提取数据
	dataInterface, ok := args["data"]
	if !ok || dataInterface == nil {
		return nil, fmt.Errorf("file_export: 缺少必需参数 'data'")
	}

	// 将数据转换为 []map[string]interface{}
	var data []map[string]interface{}
	switch v := dataInterface.(type) {
	case []interface{}:
		// 如果是 []interface{}，需要转换每个元素
		for i, item := range v {
			if itemMap, ok := item.(map[string]interface{}); ok {
				data = append(data, itemMap)
			} else {
				return nil, fmt.Errorf("file_export: data[%d] 不是有效的对象格式", i)
			}
		}
	case []map[string]interface{}:
		data = v
	default:
		return nil, fmt.Errorf("file_export: data 参数格式不正确，应为数组")
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("file_export: data 数组为空，没有数据可导出")
	}

	// 2. 提取格式参数（默认为 excel）
	format := "xlsx"
	if formatStr, ok := args["format"].(string); ok && formatStr != "" {
		format = strings.ToLower(formatStr)
		// 标准化格式名称
		switch format {
		case "excel", "xlsx":
			format = "xlsx"
		case "csv":
			format = "csv"
		case "json":
			format = "json"
		case "markdown", "md":
			format = "md"
		case "text", "txt":
			format = "txt"
		case "pdf":
			format = "pdf"
		case "docx", "word":
			format = "docx"
		default:
			return nil, fmt.Errorf("file_export: 不支持的格式 '%s'，支持的格式: excel, csv, json, markdown, text, pdf, docx", format)
		}
	}

	g.Log().Infof(ctx, "[文件导出工具] 格式: %s, 数据行数: %d", format, len(data))

	// 3. 提取列名（从第一行数据中获取）
	var columns []string
	for key := range data[0] {
		columns = append(columns, key)
	}

	// 4. 生成文件名
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("export_%s", timestamp)

	// 5. 创建导出请求
	exporter := file_export.NewFileExporter("upload")
	exportReq := &file_export.ExportRequest{
		Format:      file_export.ExportFormat(format),
		Filename:    filename,
		Columns:     columns,
		Data:        data,
		Title:       "数据导出",
		Description: fmt.Sprintf("导出时间: %s\n数据行数: %d", time.Now().Format("2006-01-02 15:04:05"), len(data)),
	}

	// 6. 执行导出
	result, err := exporter.Export(ctx, exportReq)
	if err != nil {
		g.Log().Errorf(ctx, "[文件导出工具] 导出失败: %v", err)
		return nil, fmt.Errorf("文件导出失败: %w", err)
	}

	g.Log().Infof(ctx, "[文件导出工具] 导出成功: %s (大小: %d bytes)", result.FileURL, result.Size)

	// 7. 构建返回内容
	var contentBuilder strings.Builder
	contentBuilder.WriteString(fmt.Sprintf("✅ 文件导出成功\n\n"))
	contentBuilder.WriteString(fmt.Sprintf("**文件信息:**\n"))
	contentBuilder.WriteString(fmt.Sprintf("- 文件名: %s\n", result.Filename))
	contentBuilder.WriteString(fmt.Sprintf("- 格式: %s\n", strings.ToUpper(result.Format)))
	contentBuilder.WriteString(fmt.Sprintf("- 大小: %.2f KB\n", float64(result.Size)/1024))
	contentBuilder.WriteString(fmt.Sprintf("- 行数: %d\n", result.RowCount))
	contentBuilder.WriteString(fmt.Sprintf("- 下载链接: %s\n", result.FileURL))

	return &ToolCallResult{
		Content:  contentBuilder.String(),
		FileURL:  result.FileURL,
		ToolType: "local",
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
		ToolType:   "mcp",
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
	}

	// 2. 加载历史消息（如果有 convID）
	if convID != "" {
		historyManager := history.NewManager()
		chatHistory, err := historyManager.GetHistory(convID, 100)
		if err != nil {
			g.Log().Warningf(ctx, "加载历史消息失败: %v，继续执行", err)
		} else if len(chatHistory) > 0 {
			g.Log().Infof(ctx, "[统一工具调用] 加载了 %d 条历史消息", len(chatHistory))
			messages = append(messages, chatHistory...)
		}
	}

	// 3. 添加当前用户消息
	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: e.buildFullQuestion(question, documents),
	})

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

		// 捕获 assistant 消息完成时间
		assistantMessageTime := time.Now()

		// 保存assistant消息（如果有ToolCalls且convID不为空）
		if len(response.ToolCalls) > 0 && convID != "" {
			historyManager := history.NewManager()
			if err := historyManager.SaveMessageWithMetadataAsync(response, convID, nil, &assistantMessageTime); err != nil {
				g.Log().Warningf(ctx, "保存assistant消息失败: %v", err)
				// 不阻断流程，继续执行
			} else {
				g.Log().Infof(ctx, "成功保存assistant消息，包含 %d 个工具调用", len(response.ToolCalls))
			}
		}

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

		// 标记是否有工具被执行
		hasToolExecuted := false

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
				var fileURL string
				var toolType string = "local" // 默认为 local
				if err != nil {
					resultSummary = fmt.Sprintf("工具执行失败: %v", err)
				} else if toolResult != nil {
					resultSummary = toolResult.Content
					fileURL = toolResult.FileURL
					toolType = toolResult.ToolType
					// 限制结果长度
					if len(resultSummary) > 200 {
						resultSummary = resultSummary[:200] + "..."
					}
				}
				common.WriteToolCallEnd(httpResp, messageID, toolCall.ID, toolName, toolType, resultSummary, err, duration, fileURL)
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

			// 标记工具已执行
			hasToolExecuted = true

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
			toolMessage := &schema.Message{
				Role:       schema.Tool,
				Content:    toolResult.Content,
				ToolCallID: toolCall.ID,
			}
			messages = append(messages, toolMessage)

			// 捕获 tool 消息完成时间
			toolMessageTime := time.Now()

			// 保存tool消息（如果convID不为空）
			if convID != "" {
				historyManager := history.NewManager()
				// 构建metadata，包含工具名称和参数
				metadata := map[string]interface{}{
					"tool_name": toolName,
					"tool_args": args,
				}
				if err := historyManager.SaveMessageWithMetadataAsync(toolMessage, convID, metadata, &toolMessageTime); err != nil {
					g.Log().Warningf(ctx, "保存tool消息失败: %v", err)
					// 不阻断流程，继续执行
				} else {
					g.Log().Infof(ctx, "成功保存tool消息: %s (tool_call_id: %s)", toolName, toolCall.ID)
				}
			}
		}

		// 4. 如果有工具被执行，且不是最后一轮，继续让 LLM 处理工具结果
		if hasToolExecuted && iteration < maxIterations-1 {
			g.Log().Infof(ctx, "工具执行完成，继续下一轮让 LLM 处理工具结果")
			continue
		}

		// 5. 如果是最后一轮，或者没有工具执行，强制 LLM 给出最终答案
		if iteration == maxIterations-1 || !hasToolExecuted {
			if iteration == maxIterations-1 {
				g.Log().Warning(ctx, "达到最大迭代次数，获取最终答案")
			}

			// 调用 LLM 生成最终答案（不提供工具）
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

// convertGormSkillToExecutorSkill 将 GORM Skill 模型转换为执行器 Skill
func convertGormSkillToExecutorSkill(skillModel *gormModel.ClaudeSkill) *claude_skills.Skill {
	var requirements []string
	json.Unmarshal([]byte(skillModel.Requirements), &requirements)

	var toolParameters map[string]claude_skills.SkillToolParameter
	json.Unmarshal([]byte(skillModel.ToolParameters), &toolParameters)

	var metadata map[string]interface{}
	json.Unmarshal([]byte(skillModel.Metadata), &metadata)

	return &claude_skills.Skill{
		ID:          skillModel.ID,
		Name:        skillModel.Name,
		Description: skillModel.Description,
		Version:     skillModel.Version,
		Runtime: claude_skills.SkillRuntime{
			Type:         skillModel.RuntimeType,
			Version:      skillModel.RuntimeVersion,
			Requirements: requirements,
		},
		Tool: claude_skills.SkillTool{
			Name:        skillModel.ToolName,
			Description: skillModel.ToolDescription,
			Parameters:  toolParameters,
		},
		Script:   skillModel.Script,
		Metadata: metadata,
	}
}

// executeSkillTool 执行 Claude Skill 工具
func (e *ToolExecutor) executeSkillTool(
	ctx context.Context,
	toolName string,
	args map[string]interface{},
) (*ToolCallResult, error) {

	if e.skillManager == nil {
		return nil, fmt.Errorf("Skill管理器未初始化")
	}

	// 提取 skill 名称（去掉 "skill__" 前缀）
	skillToolName := strings.TrimPrefix(toolName, "skill__")

	g.Log().Infof(ctx, "[Claude Skill] 执行 %s", skillToolName)

	// 获取 Skill
	skill, exists := e.skillManager.GetSkill(skillToolName)
	if !exists {
		return nil, fmt.Errorf("Skill 不存在: %s", skillToolName)
	}

	// 获取 HTTP Response 对象（用于发送 SSE 进度事件）
	var httpResp *ghttp.Response
	httpReq := ghttp.RequestFromCtx(ctx)
	if httpReq != nil {
		httpResp = httpReq.Response
	}

	// 获取 messageID（从 context 或其他地方）
	messageID := ""
	if httpReq != nil {
		messageID = httpReq.GetQuery("message_id").String()
	}

	// 发送 Skill 开始执行事件
	if httpResp != nil {
		common.WriteSkillProgress(httpResp, messageID, toolName, "start",
			fmt.Sprintf("开始执行 Skill: %s", skillToolName), nil)
	}

	// 创建进度回调函数
	progressCallback := func(stage string, message string, metadata map[string]interface{}) {
		if httpResp != nil {
			common.WriteSkillProgress(httpResp, messageID, toolName, stage, message, metadata)
		}
	}

	// 直接执行 Skill（带进度回调）
	result, err := e.skillManager.Executor.ExecuteSkill(ctx, skill, args, progressCallback)
	if err != nil {
		// 发送失败事件
		if httpResp != nil {
			common.WriteSkillProgress(httpResp, messageID, toolName, "error",
				fmt.Sprintf("执行失败: %v", err), nil)
		}
		return nil, fmt.Errorf("Skill执行失败: %w", err)
	}

	// 检查执行结果
	if !result.Success {
		// 发送失败事件
		if httpResp != nil {
			common.WriteSkillProgress(httpResp, messageID, toolName, "error",
				fmt.Sprintf("执行失败: %s", result.Error), nil)
		}
		return nil, fmt.Errorf("Skill执行失败: %s", result.Error)
	}

	// 发送成功事件
	if httpResp != nil {
		metadata := map[string]interface{}{
			"duration_ms": result.Duration,
		}
		common.WriteSkillProgress(httpResp, messageID, toolName, "completed",
			fmt.Sprintf("执行成功 (耗时: %dms)", result.Duration), metadata)
	}

	g.Log().Infof(ctx, "[Claude Skill] 执行成功: %s (耗时: %dms)", skillToolName, result.Duration)

	return &ToolCallResult{
		Content:  result.Output,
		ToolType: "skill",
	}, nil
}
