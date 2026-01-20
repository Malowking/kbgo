package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	coreModel "github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
	"github.com/sashabaranov/go-openai"
)

// ToolPlanGenerator 工具执行计划生成器
type ToolPlanGenerator struct {
	modelID    string
	mcpToolMap map[string]*schema.ToolInfo // MCP工具映射: mcp_service_tool -> ToolInfo
}

// NewToolPlanGenerator 创建工具执行计划生成器
func NewToolPlanGenerator(modelID string) *ToolPlanGenerator {
	return &ToolPlanGenerator{
		modelID:    modelID,
		mcpToolMap: make(map[string]*schema.ToolInfo),
	}
}

// SetMCPTools 设置MCP工具列表
func (g *ToolPlanGenerator) SetMCPTools(mcpTools []*schema.ToolInfo) {
	g.mcpToolMap = make(map[string]*schema.ToolInfo)
	for _, tool := range mcpTools {
		g.mcpToolMap[tool.Name] = tool
	}
}

// EventManager 事件管理器接口
type EventManager interface {
	SendToolPlanStart(messageID string) error
	SendToolPlanThinking(messageID, content string) error
	SendToolPlanComplete(messageID string, stepsCount int, needTools bool) error
}

// GeneratePlan  生成工具执行计划
func (g *ToolPlanGenerator) GeneratePlan(
	ctx context.Context,
	originalQuery string,
	rewrittenQuery string,
	toolConfigs []*v1.ToolConfig,
	systemPrompt string,
	chatHistory []*schema.Message,
	eventMgr EventManager,
) (*schema.ToolExecutionPlan, error) {
	// 生成消息ID
	messageID := uuid.New().String()

	// 发送开始事件
	eventMgr.SendToolPlanStart(messageID)

	// 调用 LLM
	mc := coreModel.Registry.GetChatModel(g.modelID)
	if mc == nil {
		return nil, fmt.Errorf("model not found: %s", g.modelID)
	}

	modelService := coreModel.NewModelService(mc.APIKey, mc.BaseURL, nil)

	// ========== 第一步：生成思考过程（流式，使用原始query） ==========
	thinkingPrompt := g.buildThinkingPrompt(toolConfigs, systemPrompt, chatHistory)
	thinkingMessages := []*schema.Message{
		{
			Role:    schema.System,
			Content: thinkingPrompt,
		},
		{
			Role:    schema.User,
			Content: originalQuery, // 使用原始query，让用户看到模型在思考原始问题
		},
	}

	thinkingParams := coreModel.ChatCompletionParams{
		ModelName:   mc.Name,
		Messages:    thinkingMessages,
		Temperature: 0.7,
	}

	thinkingStream, err := modelService.ChatCompletionStream(ctx, thinkingParams)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM for thinking: %w", err)
	}

	// 收集思考过程并流式发送
	var thinkingContent strings.Builder
	for {
		response, err := thinkingStream.Recv()
		if err != nil {
			break
		}

		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta.Content
			if delta != "" {
				thinkingContent.WriteString(delta)
				// 实时发送思考内容
				eventMgr.SendToolPlanThinking(messageID, delta)
			}
		}
	}
	thinkingStream.Close()

	reasoning := thinkingContent.String()

	// ========== 第二步：基于思考生成 JSON 计划（使用重写后的query） ==========
	planPrompt := g.buildJSONPlanPrompt(toolConfigs, reasoning)
	planMessages := []*schema.Message{
		{
			Role:    schema.System,
			Content: planPrompt,
		},
		{
			Role:    schema.User,
			Content: rewrittenQuery, // 使用重写后的query，用于生成更准确的工具调用计划
		},
	}

	planParams := coreModel.ChatCompletionParams{
		ModelName:   mc.Name,
		Messages:    planMessages,
		Temperature: 0.7,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	}

	planStream, err := modelService.ChatCompletionStream(ctx, planParams)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM for plan: %w", err)
	}
	defer planStream.Close()

	// 收集 JSON 响应
	var fullContent strings.Builder
	for {
		response, err := planStream.Recv()
		if err != nil {
			break
		}

		if len(response.Choices) > 0 {
			delta := response.Choices[0].Delta.Content
			if delta != "" {
				fullContent.WriteString(delta)
			}
		}
	}

	// 解析响应
	content := fullContent.String()
	plan, err := g.parsePlanResponse(ctx, messageID, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse plan response: %w", err)
	}

	// 使用第一步的思考内容作为 reasoning
	plan.Reasoning = reasoning

	// 为每个步骤添加工具配置参数
	g.enrichStepsWithToolConfig(ctx, plan, toolConfigs, rewrittenQuery)

	// 发送完成事件
	eventMgr.SendToolPlanComplete(messageID, len(plan.Steps), plan.NeedTools)

	return plan, nil
}

// buildToolDescriptions 构建工具描述
func (g *ToolPlanGenerator) buildToolDescriptions(toolConfigs []*v1.ToolConfig) string {
	var builder strings.Builder

	toolDescriptions := map[string]string{
		"knowledge_retrieval": "知识库检索工具：从知识库中检索相关文档和信息",
		"nl2sql":              "数据查询工具：将自然语言转换为SQL查询并执行",
		"file_export":         "文件导出工具：将数据导出为文件（Excel、CSV等）",
		"claude_skills":       "自定义脚本工具：执行自定义的Python脚本",
	}

	for _, toolConfig := range toolConfigs {
		if !toolConfig.Enabled {
			continue
		}

		if toolConfig.Type == "local_tools" && toolConfig.Config != nil {
			builder.WriteString("\n### 本地工具（tool_type: local_tools）：\n")
			for toolName, toolConfigValue := range toolConfig.Config {
				desc := toolDescriptions[toolName]
				if desc == "" {
					desc = toolName
				}

				builder.WriteString(fmt.Sprintf("- **%s**: %s\n", toolName, desc))

				if configMap, ok := toolConfigValue.(map[string]interface{}); ok && len(configMap) > 0 {
					builder.WriteString("  参数：\n")
					for key := range configMap {
						builder.WriteString(fmt.Sprintf("  - %s\n", key))
					}
				}
			}
			continue
		}

		// 特殊处理 MCP 工具类型
		if toolConfig.Type == "mcp" {
			// 添加所有 MCP 工具的详细信息
			if len(g.mcpToolMap) > 0 {
				builder.WriteString("\n### MCP 工具（外部服务）：\n")
				for toolName, toolInfo := range g.mcpToolMap {
					builder.WriteString(fmt.Sprintf("- **%s**: %s\n", toolName, toolInfo.Desc))

					// 添加参数说明
					if toolInfo.ParamsOneOf != nil {
						// 将 ParamsOneOf 转换为 OpenAPI 格式以获取参数信息
						openAPISchema, err := toolInfo.ParamsOneOf.ToOpenAPIV3()
						if err == nil && openAPISchema != nil {
							if schemaMap, ok := openAPISchema.(map[string]interface{}); ok {
								if properties, ok := schemaMap["properties"].(map[string]interface{}); ok && len(properties) > 0 {
									builder.WriteString("  参数：\n")

									// 获取必需参数列表
									requiredParams := make(map[string]bool)
									if required, ok := schemaMap["required"].([]string); ok {
										for _, r := range required {
											requiredParams[r] = true
										}
									}

									// 遍历参数
									for paramName, paramDefRaw := range properties {
										if paramDef, ok := paramDefRaw.(map[string]interface{}); ok {
											requiredStr := ""
											if requiredParams[paramName] {
												requiredStr = "（必需）"
											}
											desc := ""
											if d, ok := paramDef["description"].(string); ok {
												desc = d
											}
											builder.WriteString(fmt.Sprintf("    - %s%s: %s\n", paramName, requiredStr, desc))
										}
									}
								}
							}
						}
					}
				}
			} else {
				builder.WriteString("- **mcp**: 外部服务调用工具（无可用服务）\n")
			}
			continue
		}

		// 处理其他工具类型
		desc := toolDescriptions[toolConfig.Type]
		if desc == "" {
			desc = toolConfig.Type
		}

		builder.WriteString(fmt.Sprintf("- **%s**: %s\n", toolConfig.Type, desc))

		// 添加工具的配置参数说明
		if toolConfig.Config != nil && len(toolConfig.Config) > 0 {
			builder.WriteString("  参数：\n")
			for key := range toolConfig.Config {
				builder.WriteString(fmt.Sprintf("  - %s\n", key))
			}
		}
	}

	return builder.String()
}

// parsePlanResponse 解析计划响应
func (gen *ToolPlanGenerator) parsePlanResponse(ctx context.Context, messageID, content string) (*schema.ToolExecutionPlan, error) {
	// 提取JSON内容（处理可能的markdown代码块或其他格式）
	content = gen.extractJSON(content)

	// 解析 JSON
	var response struct {
		NeedTools bool   `json:"need_tools"`
		Reasoning string `json:"reasoning"`
		Steps     []struct {
			StepID    string   `json:"step_id"`
			ToolName  string   `json:"tool_name"`
			ToolType  string   `json:"tool_type"`
			Reason    string   `json:"reason"`
			DependsOn []string `json:"depends_on"`
		} `json:"steps"`
	}

	if err := json.Unmarshal([]byte(content), &response); err != nil {
		g.Log().Errorf(ctx, "Failed to parse JSON, raw content: %s", content)
		return nil, fmt.Errorf("failed to unmarshal JSON: %w", err)
	}

	// 创建计划
	plan := schema.NewToolExecutionPlan(messageID)
	plan.NeedTools = response.NeedTools
	plan.Reasoning = response.Reasoning

	// 添加步骤（不包含 parameters，由后续 tool_call 生成）
	for _, stepData := range response.Steps {
		step := schema.NewToolExecutionStep(stepData.StepID, stepData.ToolName, stepData.ToolType)
		step.Reason = stepData.Reason
		step.DependsOn = stepData.DependsOn
		plan.AddStep(step)
	}

	return plan, nil
}

// extractJSON 从响应中提取JSON内容
func (g *ToolPlanGenerator) extractJSON(content string) string {
	// 移除可能的markdown代码块标记
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)
	start := strings.Index(content, "{")
	end := strings.LastIndex(content, "}")

	if start != -1 && end != -1 && end > start {
		return content[start : end+1]
	}

	return content
}

// ValidatePlan 验证工具执行计划
func (g *ToolPlanGenerator) ValidatePlan(plan *schema.ToolExecutionPlan, toolConfigs []*v1.ToolConfig) error {
	if !plan.NeedTools {
		return nil
	}

	// 构建可用工具映射
	availableTools := make(map[string]bool)
	availableLocalTools := make(map[string]bool)
	for _, toolConfig := range toolConfigs {
		if toolConfig.Enabled {
			availableTools[toolConfig.Type] = true
			if toolConfig.Type == "local_tools" && toolConfig.Config != nil {
				for toolName := range toolConfig.Config {
					availableLocalTools[toolName] = true
				}
			}
		}
	}

	// 验证每个步骤
	stepIDs := make(map[string]bool)
	for _, step := range plan.Steps {
		// 检查步骤ID是否唯一
		if stepIDs[step.StepID] {
			return fmt.Errorf("duplicate step_id: %s", step.StepID)
		}
		stepIDs[step.StepID] = true

		// 检查工具是否可用
		if !availableTools[step.ToolType] {
			return fmt.Errorf("tool not available: %s", step.ToolType)
		}
		if step.ToolType == "local_tools" && !availableLocalTools[step.ToolName] {
			return fmt.Errorf("local tool not available: %s", step.ToolName)
		}

		// 检查依赖的步骤是否存在
		for _, depID := range step.DependsOn {
			if !stepIDs[depID] {
				// 依赖的步骤必须在当前步骤之前定义
				found := false
				for _, s := range plan.Steps {
					if s.StepID == depID {
						found = true
						break
					}
					if s.StepID == step.StepID {
						break
					}
				}
				if !found {
					return fmt.Errorf("step %s depends on non-existent step: %s", step.StepID, depID)
				}
			}
		}
	}

	return nil
}

// LogPlan 记录工具执行计划
func (gen *ToolPlanGenerator) LogPlan(ctx context.Context, plan *schema.ToolExecutionPlan) {
	g.Log().Infof(ctx, "[ToolPlanGenerator] Generated plan:")
	g.Log().Infof(ctx, "  Message ID: %s", plan.MessageID)
	g.Log().Infof(ctx, "  Need Tools: %v", plan.NeedTools)
	g.Log().Infof(ctx, "  Reasoning: %s", plan.Reasoning)
	g.Log().Infof(ctx, "  Steps: %d", len(plan.Steps))
	for i, step := range plan.Steps {
		g.Log().Infof(ctx, "    Step %d: %s (%s)", i+1, step.StepID, step.ToolType)
		g.Log().Infof(ctx, "      Reason: %s", step.Reason)
		if len(step.DependsOn) > 0 {
			g.Log().Infof(ctx, "      Depends on: %v", step.DependsOn)
		}
	}
}

// buildThinkingPrompt 构建思考过程的提示词（第一步）
func (g *ToolPlanGenerator) buildThinkingPrompt(
	toolConfigs []*v1.ToolConfig,
	systemPrompt string,
	chatHistory []*schema.Message,
) string {
	var builder strings.Builder

	builder.WriteString("你是一个智能助手，需要分析用户的问题并思考如何解决。\n\n")

	// 添加可用工具列表
	builder.WriteString("## 可用工具：\n")
	builder.WriteString(g.buildToolDescriptions(toolConfigs))
	builder.WriteString("\n")

	// 添加 Agent 角色定位
	if systemPrompt != "" {
		builder.WriteString("## Agent 角色定位：\n")
		builder.WriteString(systemPrompt)
		builder.WriteString("\n\n")
	}

	// 添加对话历史
	if len(chatHistory) > 0 {
		builder.WriteString("## 对话历史：\n")
		for _, msg := range chatHistory {
			builder.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
		}
		builder.WriteString("\n")
	}

	builder.WriteString(`## 任务说明

请基于对话历史分析最后的用户的问题，并详细说明你的思考过程：
1. 用户想要什么？
2. 是否需要使用工具？如果需要，应该使用哪些工具？为什么？
3. 工具的执行顺序是什么？

-注意：不需要分析该工具参数相关问题！！！
请用自然语言详细描述你的分析和推理过程，只能输出文字内容，格式为markdown格式文档，不要有任何代码生成！！！
`)

	return builder.String()
}

// enrichStepsWithToolConfig 为每个步骤添加工具配置参数
func (gen *ToolPlanGenerator) enrichStepsWithToolConfig(ctx context.Context, plan *schema.ToolExecutionPlan, toolConfigs []*v1.ToolConfig, userQuery string) {
	// 构建工具配置映射
	toolConfigMap := make(map[string]map[string]interface{})

	for _, toolConfig := range toolConfigs {
		if !toolConfig.Enabled {
			continue
		}

		// 处理本地工具
		if toolConfig.Type == "local_tools" && toolConfig.Config != nil {
			for toolName, configValue := range toolConfig.Config {
				if configMap, ok := configValue.(map[string]interface{}); ok {
					toolConfigMap[toolName] = configMap
				}
			}
		}

		// 处理 MCP 工具
		if toolConfig.Type == "mcp" && toolConfig.Config != nil {
			// MCP 工具的配置可能包含服务级别的配置
			// 这里暂时不处理，因为 MCP 工具的参数由 LLM 生成
		}
	}

	// 为每个步骤添加配置参数
	for _, step := range plan.Steps {
		if step.Parameters == nil {
			step.Parameters = make(map[string]interface{})
		}

		// 如果是本地工具，添加配置参数
		if step.ToolType == "local_tools" {
			// 1. 添加 query 参数
			step.Parameters["query"] = userQuery

			// 2. 添加工具特定的配置参数
			if config, ok := toolConfigMap[step.ToolName]; ok {
				// 将配置参数合并到 step.Parameters
				for key, value := range config {
					step.Parameters[key] = value
				}
			}

			// 3. 添加 model_id 参数
			step.Parameters["model_id"] = gen.modelID
		}
	}
}

// buildJSONPlanPrompt 构建 JSON 计划的提示词
func (g *ToolPlanGenerator) buildJSONPlanPrompt(
	toolConfigs []*v1.ToolConfig,
	reasoning string,
) string {
	var builder strings.Builder

	builder.WriteString("你是一个智能助手，需要根据之前的分析生成工具执行计划。\n\n")

	// 添加可用工具列表
	builder.WriteString("## 可用工具：\n")
	builder.WriteString(g.buildToolDescriptions(toolConfigs))
	builder.WriteString("\n")

	// 添加之前的思考过程
	builder.WriteString("## 你的分析过程：\n")
	builder.WriteString(reasoning)
	builder.WriteString("\n\n")

	builder.WriteString(`## 任务说明

根据上面的分析，请生成一个 JSON 格式的工具执行计划。

**重要：只需要输出纯JSON格式，不要添加任何额外的文字说明或markdown代码块标记。**
**注意：不需要生成工具的参数（parameters），只需要生成工具的执行顺序和依赖关系。工具参数将由后续的 tool_call 机制自动生成。**

JSON格式如下：

{
  "need_tools": true/false,
  "steps": [
    {
      "step_id": "1",
      "tool_name": "knowledge_retrieval",
      "tool_type": "local_tools",
      "reason": "需要从知识库检索相关信息",
      "depends_on": []
    },
    {
      "step_id": "2",
      "tool_name": "nl2sql",
      "tool_type": "local_tools",
      "reason": "需要查询数据库获取数据",
      "depends_on": ["step_1"]
    }
  ]
}

注意：
- step_id 必须唯一，必须使用 1, 2, 3 等格式
- depends_on 是一个数组，包含依赖的步骤ID，表示当前步骤需要等待哪些步骤完成后才能执行
- 如果不需要使用工具，need_tools 设置为 false，steps 设置为空数组
- 对于本地工具，tool_type 必须是 "local_tools"，tool_name 是具体工具名称（如 knowledge_retrieval、nl2sql、file_export）
- 对于MCP工具，tool_name 应该是完整的工具名称（如 mcp_service_tool），tool_type 应该是 "mcp"

`)

	return builder.String()
}
