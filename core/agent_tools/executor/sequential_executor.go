package executor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools/registry"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/Malowking/kbgo/internal/history"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/sashabaranov/go-openai"
)

// ToolManager 工具管理器接口
type ToolManager interface {
	ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}) (*schema.ToolResult, error)
}

// StreamEventManager 流式事件管理器接口
type StreamEventManager interface {
	SendToolExecutionStart(messageID, stepID, toolName, reason string) error
	SendToolExecutionComplete(messageID, stepID, toolName, resultSummary string) error
	SendToolExecutionError(messageID, stepID, toolName, errorMsg string) error
}

// SequentialToolExecutor 顺序工具执行器
type SequentialToolExecutor struct {
	modelID        string
	toolManager    ToolManager
	eventMgr       StreamEventManager
	toolRegistry   registry.ToolRegistry
	historyManager *history.Manager
	traceID        string // 链路追踪ID
}

// NewSequentialToolExecutor 创建顺序工具执行器
func NewSequentialToolExecutor(
	modelID string,
	toolManager ToolManager,
	eventMgr StreamEventManager,
	toolRegistry registry.ToolRegistry,
	historyManager *history.Manager,
	traceID string,
) *SequentialToolExecutor {
	return &SequentialToolExecutor{
		modelID:        modelID,
		toolManager:    toolManager,
		eventMgr:       eventMgr,
		toolRegistry:   toolRegistry,
		historyManager: historyManager,
		traceID:        traceID,
	}
}

// ExecuteToolsWithPlan 按计划顺序执行工具
func (e *SequentialToolExecutor) ExecuteToolsWithPlan(
	ctx context.Context,
	plan *schema.ToolExecutionPlan,
	toolConfigs []*v1.ToolConfig,
	convID string,
	modelID string,
	originalQuery string,
) ([]*schema.Message, []*schema.ToolResult, error) {
	g.Log().Infof(ctx, "[SequentialToolExecutor] Starting sequential execution, steps: %d", len(plan.Steps))

	// 初始化上下文管理器
	contextMgr := NewContextManager(50, 100000) // 最多50条消息，10万token

	// 添加用户原始问题到上下文
	userMsg := &schema.Message{
		Role:    schema.User,
		Content: originalQuery,
	}
	contextMgr.AddMessage(userMsg)

	// 用于收集所有消息和结果
	var allMessages []*schema.Message
	var allResults []*schema.ToolResult

	// 遍历计划中的每个步骤
	for i, step := range plan.Steps {
		g.Log().Infof(ctx, "[SequentialToolExecutor] Executing step %d/%d: %s", i+1, len(plan.Steps), step.ToolName)

		// 发送工具执行开始事件
		if e.eventMgr != nil {
			e.eventMgr.SendToolExecutionStart(plan.MessageID, step.StepID, step.ToolName, step.Reason)
		}

		// 标记步骤为执行中
		step.MarkAsRunning()

		// 构建步骤提示词
		stepPrompt := e.buildStepPrompt(step, i+1, len(plan.Steps))
		stepMsg := &schema.Message{
			Role:    schema.User,
			Content: stepPrompt,
		}
		contextMgr.AddMessage(stepMsg)

		// 构建工具定义
		toolDef, err := e.buildToolDefinition(step, toolConfigs)
		if err != nil {
			g.Log().Errorf(ctx, "[SequentialToolExecutor] Failed to build tool definition: %v", err)
			step.MarkAsFailed(err)
			if e.eventMgr != nil {
				e.eventMgr.SendToolExecutionError(plan.MessageID, step.StepID, step.ToolName, err.Error())
			}
			continue
		}

		// 调用 LLM 获取 tool_calls
		assistantMsg, err := e.callLLMWithTool(ctx, contextMgr.GetMessages(), modelID, []openai.Tool{toolDef})
		if err != nil {
			g.Log().Errorf(ctx, "[SequentialToolExecutor] Failed to call LLM: %v", err)
			step.MarkAsFailed(err)
			if e.eventMgr != nil {
				e.eventMgr.SendToolExecutionError(plan.MessageID, step.StepID, step.ToolName, err.Error())
			}
			continue
		}

		// 打印 assistantMsg
		assistantMsgJSON, err := json.Marshal(assistantMsg)
		if err != nil {
			g.Log().Errorf(ctx, "[SequentialToolExecutor] Failed to marshal assistantMsg: %v", err)
		} else {
			g.Log().Infof(ctx, "[SequentialToolExecutor] ========== assistantMsg: %s", string(assistantMsgJSON))
		}

		// 检查是否有 tool_calls
		if len(assistantMsg.ToolCalls) == 0 {
			err := fmt.Errorf("LLM did not return tool calls")
			g.Log().Errorf(ctx, "[SequentialToolExecutor] %v", err)
			step.MarkAsFailed(err)
			if e.eventMgr != nil {
				e.eventMgr.SendToolExecutionError(plan.MessageID, step.StepID, step.ToolName, err.Error())
			}
			continue
		}

		// 保存 assistant 消息（包含 tool_calls）
		allMessages = append(allMessages, assistantMsg)
		contextMgr.AddMessage(assistantMsg)

		// 保存 assistant 消息到数据库
		now := time.Now()
		if err := e.historyManager.SaveMessage(assistantMsg, convID, nil, &now, e.traceID); err != nil {
			g.Log().Warningf(ctx, "[SequentialToolExecutor] Failed to save assistant message: %v", err)
		}

		// 执行工具调用（通常只有一个）
		for _, toolCall := range assistantMsg.ToolCalls {
			// 执行工具
			toolResult, toolMsg, err := e.executeToolCall(ctx, toolCall, step, toolConfigs)
			if err != nil {
				g.Log().Errorf(ctx, "[SequentialToolExecutor] Failed to execute tool: %v", err)
				step.MarkAsFailed(err)
				if e.eventMgr != nil {
					e.eventMgr.SendToolExecutionError(plan.MessageID, step.StepID, step.ToolName, err.Error())
				}
				continue
			}

			// 标记步骤为完成
			step.MarkAsCompleted(toolResult)

			// 收集结果
			allResults = append(allResults, toolResult)
			allMessages = append(allMessages, toolMsg)

			// 将 tool 消息加入上下文
			contextMgr.AddMessage(toolMsg)

			// 保存 tool 消息到数据库
			now := time.Now()
			if err := e.historyManager.SaveMessage(toolMsg, convID, nil, &now, e.traceID); err != nil {
				g.Log().Warningf(ctx, "[SequentialToolExecutor] Failed to save tool message: %v", err)
			}

			// 发送工具执行完成事件
			if e.eventMgr != nil {
				resultSummary := e.buildResultSummary(toolResult)
				e.eventMgr.SendToolExecutionComplete(plan.MessageID, step.StepID, step.ToolName, resultSummary)
			}

			g.Log().Infof(ctx, "[SequentialToolExecutor] Step %d completed successfully", i+1)
		}
	}

	g.Log().Infof(ctx, "[SequentialToolExecutor] Sequential execution completed, total messages: %d, results: %d",
		len(allMessages), len(allResults))

	return allMessages, allResults, nil
}

// buildStepPrompt 构建步骤提示词
func (e *SequentialToolExecutor) buildStepPrompt(step *schema.ToolExecutionStep, currentStep, totalSteps int) string {
	prompt := fmt.Sprintf("请执行第 %d/%d 步工具调用：\n", currentStep, totalSteps)
	prompt += fmt.Sprintf("工具名称：%s\n", step.ToolName)
	prompt += fmt.Sprintf("执行原因：%s\n", step.Reason)

	// 如果有参数，添加参数信息
	if len(step.Parameters) > 0 {
		paramsJSON, _ := json.Marshal(step.Parameters)
		prompt += fmt.Sprintf("\n必需参数（请直接使用这些参数调用工具）：%s\n", string(paramsJSON))
		prompt += "\n重要提示：上述参数是系统配置的必需参数，请直接使用它们调用工具，不要向用户确认。"
	}

	prompt += "\n\n请立即使用提供的工具和参数完成此步骤。"

	return prompt
}

// buildToolDefinition 构建工具定义
func (e *SequentialToolExecutor) buildToolDefinition(
	step *schema.ToolExecutionStep,
	toolConfigs []*v1.ToolConfig,
) (openai.Tool, error) {
	// 从注册表获取工具定义
	_, err := e.toolRegistry.GetToolDefinition(step.ToolName)
	if err != nil {
		// 如果找不到，创建一个基本定义
		g.Log().Warningf(context.Background(), "[SequentialToolExecutor] Tool %s not found in registry, creating fallback definition", step.ToolName)
		return e.createFallbackToolDefinition(step), nil
	}

	// 转换为 OpenAI 格式
	openaiTool, err := e.toolRegistry.ConvertToOpenAITool(step.ToolName)
	if err != nil {
		g.Log().Warningf(context.Background(), "[SequentialToolExecutor] Failed to convert tool %s, creating fallback definition", step.ToolName)
		return e.createFallbackToolDefinition(step), nil
	}

	// 可以根据 step.Reason 调整描述
	if step.Reason != "" {
		openaiTool.Function.Description = fmt.Sprintf("%s\n执行原因：%s",
			openaiTool.Function.Description, step.Reason)
	}

	return *openaiTool, nil
}

// createFallbackToolDefinition 创建备用工具定义
func (e *SequentialToolExecutor) createFallbackToolDefinition(step *schema.ToolExecutionStep) openai.Tool {
	// 构建基本的参数 schema
	params := map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
		"required":   []string{},
	}

	// 从 step.Parameters 推断参数定义
	if len(step.Parameters) > 0 {
		properties := make(map[string]interface{})
		for key := range step.Parameters {
			properties[key] = map[string]interface{}{
				"type":        "string",
				"description": fmt.Sprintf("Parameter %s", key),
			}
		}
		params["properties"] = properties
	}

	return openai.Tool{
		Type: openai.ToolTypeFunction,
		Function: &openai.FunctionDefinition{
			Name:        step.ToolName,
			Description: fmt.Sprintf("Execute %s tool. Reason: %s", step.ToolName, step.Reason),
			Parameters:  params,
		},
	}
}

// callLLMWithTool 调用 LLM 获取 tool_calls
func (e *SequentialToolExecutor) callLLMWithTool(
	ctx context.Context,
	messages []*schema.Message,
	modelID string,
	tools []openai.Tool,
) (*schema.Message, error) {
	g.Log().Infof(ctx, "[SequentialToolExecutor] Calling LLM with %d messages and %d tools", len(messages), len(tools))

	// 创建模型服务
	mc := model.Registry.GetChatModel(modelID)
	if mc == nil {
		return nil, fmt.Errorf("model not found: %s", modelID)
	}

	modelService := model.NewModelService(mc.APIKey, mc.BaseURL, nil)

	// 构建请求参数
	params := model.ChatCompletionParams{
		ModelName:   mc.Name,
		Messages:    messages,
		Temperature: 0.7,
		Tools:       tools,
		ToolChoice:  "auto",
	}

	// 调用 LLM
	response, err := modelService.ChatCompletion(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("failed to call LLM: %w", err)
	}

	// 检查响应
	if len(response.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	choice := response.Choices[0]

	// 构建 assistant 消息
	assistantMsg := &schema.Message{
		Role:    schema.Assistant,
		Content: choice.Message.Content,
	}

	// 转换 tool_calls
	if len(choice.Message.ToolCalls) > 0 {
		assistantMsg.ToolCalls = make([]schema.ToolCall, len(choice.Message.ToolCalls))
		for i, tc := range choice.Message.ToolCalls {
			assistantMsg.ToolCalls[i] = schema.ToolCall{
				ID:   tc.ID,
				Type: string(tc.Type),
				Function: schema.FunctionCall{
					Name:      tc.Function.Name,
					Arguments: tc.Function.Arguments,
				},
			}
		}
	}

	g.Log().Infof(ctx, "[SequentialToolExecutor] LLM returned %d tool calls", len(assistantMsg.ToolCalls))

	return assistantMsg, nil
}

// executeToolCall 执行单个工具调用
func (e *SequentialToolExecutor) executeToolCall(
	ctx context.Context,
	toolCall schema.ToolCall,
	step *schema.ToolExecutionStep,
	toolConfigs []*v1.ToolConfig,
) (*schema.ToolResult, *schema.Message, error) {
	g.Log().Infof(ctx, "[SequentialToolExecutor] Executing tool call: %s (ID: %s)", toolCall.Function.Name, toolCall.ID)

	// 解析参数
	var arguments map[string]interface{}
	if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &arguments); err != nil {
		return nil, nil, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// 合并计划中的参数和 LLM 返回的参数
	finalParams := make(map[string]interface{})
	for k, v := range step.Parameters {
		finalParams[k] = v
	}
	for k, v := range arguments {
		finalParams[k] = v
	}

	// 执行工具
	toolResult, err := e.toolManager.ExecuteTool(ctx, step.ToolName, finalParams)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute tool: %w", err)
	}

	// 构建 tool 消息（使用真实的 tool_call_id）
	// 将 Data 转换为字符串
	var dataStr string
	if toolResult.Data != nil {
		dataBytes, err := json.Marshal(toolResult.Data)
		if err != nil {
			dataStr = fmt.Sprintf("%v", toolResult.Data)
		} else {
			dataStr = string(dataBytes)
		}
	}

	toolMsg := &schema.Message{
		Role:       schema.Tool,
		Content:    dataStr,
		ToolCallID: toolCall.ID,
	}

	g.Log().Infof(ctx, "[SequentialToolExecutor] Tool execution completed: %s", step.ToolName)

	return toolResult, toolMsg, nil
}

// buildResultSummary 构建结果摘要
func (e *SequentialToolExecutor) buildResultSummary(result *schema.ToolResult) string {
	if result == nil {
		return "无结果"
	}

	// 将 Data 转换为字符串
	var summary string
	if result.Data != nil {
		dataBytes, err := json.Marshal(result.Data)
		if err != nil {
			summary = fmt.Sprintf("%v", result.Data)
		} else {
			summary = string(dataBytes)
		}
	}

	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}

	return summary
}

// ContextManager 上下文管理器
type ContextManager struct {
	messages    []*schema.Message
	maxMessages int
	maxTokens   int
}

// NewContextManager 创建上下文管理器
func NewContextManager(maxMessages, maxTokens int) *ContextManager {
	return &ContextManager{
		messages:    make([]*schema.Message, 0),
		maxMessages: maxMessages,
		maxTokens:   maxTokens,
	}
}

// AddMessage 添加消息到上下文
func (cm *ContextManager) AddMessage(msg *schema.Message) {
	cm.messages = append(cm.messages, msg)

	// 如果超过限制，进行截断
	if len(cm.messages) > cm.maxMessages {
		cm.truncate()
	}
}

// GetMessages 获取当前上下文
func (cm *ContextManager) GetMessages() []*schema.Message {
	return cm.messages
}

// truncate 截断上下文（保留最近的消息）
func (cm *ContextManager) truncate() {
	// 简单策略：保留最近的 maxMessages 条消息
	if len(cm.messages) > cm.maxMessages {
		// 保留第一条（通常是用户原始问题）和最近的消息
		firstMsg := cm.messages[0]
		recentMsgs := cm.messages[len(cm.messages)-cm.maxMessages+1:]
		cm.messages = append([]*schema.Message{firstMsg}, recentMsgs...)
	}
}
