package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/sashabaranov/go-openai"
)

// countTotalTools 统计工具总数
func (h *ChatHandler) countTotalTools(mcpServiceTools map[string][]string) int {
	if mcpServiceTools == nil {
		return 0
	}

	count := 0
	for _, tools := range mcpServiceTools {
		count += len(tools)
	}
	return count
}

// buildToolSelectionQuestion 构建用于工具选择的完整问题
func (h *ChatHandler) buildToolSelectionQuestion(ctx context.Context, question string, documents []*schema.Document, fileContent string) string {
	var builder strings.Builder

	// 基础问题
	builder.WriteString("用户问题：")
	builder.WriteString(question)
	builder.WriteString("\n\n")

	// 如果有知识库检索内容
	if len(documents) > 0 {
		builder.WriteString("知识库检索到的相关内容：\n")
		for i, doc := range documents {
			if i >= 3 { // 最多包含3个文档片段
				break
			}
			builder.WriteString(fmt.Sprintf("文档%d：%s\n", i+1, doc.Content))
		}
		builder.WriteString("\n")
	}

	// 如果有文件解析内容
	if fileContent != "" {
		// 限制文件内容长度
		maxLen := 1000
		content := fileContent
		if len(content) > maxLen {
			content = content[:maxLen] + "..."
		}
		builder.WriteString("文件解析内容：\n")
		builder.WriteString(content)
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// selectRandomLLMModel 随机选择一个LLM模型
func (h *ChatHandler) selectRandomLLMModel(ctx context.Context) (string, error) {
	// 获取所有LLM类型的模型
	llmModels := model.Registry.GetByType(model.ModelTypeLLM)
	if len(llmModels) == 0 {
		return "", fmt.Errorf("没有可用的LLM模型")
	}

	// 随机选择一个模型
	rand.Seed(time.Now().UnixNano())
	selectedModel := llmModels[rand.Intn(len(llmModels))]

	g.Log().Infof(ctx, "随机选择LLM模型: %s (%s)", selectedModel.Name, selectedModel.ModelID)
	return selectedModel.ModelID, nil
}

// getAllMCPTools 获取数据库中所有MCP工具
func (h *ChatHandler) getAllMCPTools(ctx context.Context) (map[string][]v1.MCPToolInfo, error) {
	// 获取所有启用的MCP服务
	registries, _, err := dao.MCPRegistry.List(ctx, nil, 1, 100)
	if err != nil {
		return nil, fmt.Errorf("获取MCP服务列表失败: %w", err)
	}

	allTools := make(map[string][]v1.MCPToolInfo)

	for _, registry := range registries {
		// 只处理启用的服务
		if registry.Status != 1 {
			continue
		}

		// 从数据库缓存获取工具列表
		if registry.Tools != "" && registry.Tools != "[]" {
			var toolInfos []v1.MCPToolInfo
			if err := json.Unmarshal([]byte(registry.Tools), &toolInfos); err == nil {
				allTools[registry.Name] = toolInfos
				g.Log().Debugf(ctx, "从服务 %s 加载了 %d 个工具", registry.Name, len(toolInfos))
			}
		}
	}

	return allTools, nil
}

// buildToolSelectionPrompt 构建工具选择的prompt
func (h *ChatHandler) buildToolSelectionPrompt(ctx context.Context, question string, allTools map[string][]v1.MCPToolInfo) string {
	var builder strings.Builder

	builder.WriteString("你是一个智能工具选择助手。根据用户问题和可用的工具列表，选择最合适的工具来回答用户问题。\n\n")
	builder.WriteString("用户问题：\n")
	builder.WriteString(question)
	builder.WriteString("\n\n")

	builder.WriteString("可用的工具列表：\n")
	for serviceName, tools := range allTools {
		builder.WriteString(fmt.Sprintf("\n服务名称：%s\n", serviceName))
		for _, tool := range tools {
			builder.WriteString(fmt.Sprintf("  - 工具名：%s\n", tool.Name))
			builder.WriteString(fmt.Sprintf("    描述：%s\n", tool.Description))
		}
	}

	builder.WriteString("\n请根据用户问题选择最合适的工具（最多选择5个工具）。\n")
	builder.WriteString("要求：\n")
	builder.WriteString("1. 只选择与问题相关的工具\n")
	builder.WriteString("2. 如果问题不需要使用任何工具，返回空对象 {}\n")
	builder.WriteString("3. 返回格式必须是JSON对象，键是服务名，值是该服务下的工具名数组\n")
	builder.WriteString("4. 示例格式：{\"服务名1\": [\"工具名1\", \"工具名2\"], \"服务名2\": [\"工具名3\"]}\n")

	return builder.String()
}

// selectToolsWithLLM 使用LLM选择工具
func (h *ChatHandler) selectToolsWithLLM(ctx context.Context, question string) (map[string][]string, error) {
	// 1. 获取所有可用的MCP工具
	allTools, err := h.getAllMCPTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取MCP工具列表失败: %w", err)
	}

	if len(allTools) == 0 {
		g.Log().Info(ctx, "没有可用的MCP工具")
		return nil, nil
	}

	g.Log().Infof(ctx, "加载了 %d 个MCP服务的工具", len(allTools))

	// 2. 随机选择一个LLM模型
	modelID, err := h.selectRandomLLMModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("选择LLM模型失败: %w", err)
	}

	// 获取模型配置
	mc := model.Registry.Get(modelID)
	if mc == nil {
		return nil, fmt.Errorf("模型不存在: %s", modelID)
	}

	// 3. 构建工具选择的prompt
	prompt := h.buildToolSelectionPrompt(ctx, question, allTools)
	g.Log().Debugf(ctx, "工具选择prompt长度: %d", len(prompt))

	// 4. 构建请求消息
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleUser,
			Content: prompt,
		},
	}

	// 5. 调用LLM，使用 ResponseFormat 强制返回 JSON
	chatReq := openai.ChatCompletionRequest{
		Model:    mc.Name,
		Messages: messages,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
		Temperature: 0.3, // 使用较低的温度以获得更稳定的输出
	}

	resp, err := mc.Client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return nil, fmt.Errorf("LLM调用失败: %w", err)
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("LLM返回空响应")
	}

	responseContent := resp.Choices[0].Message.Content
	g.Log().Debugf(ctx, "LLM工具选择响应: %s", responseContent)

	// 6. 解析LLM的输出（使用 ResponseFormat 后应该直接是 JSON）
	selectedTools, err := h.parseToolSelectionResponse(ctx, responseContent)
	if err != nil {
		return nil, fmt.Errorf("解析工具选择响应失败: %w", err)
	}

	return selectedTools, nil
}

// parseToolSelectionResponse 解析LLM的工具选择响应
func (h *ChatHandler) parseToolSelectionResponse(ctx context.Context, response string) (map[string][]string, error) {
	response = strings.TrimSpace(response)

	// 使用 ResponseFormat 后，响应应该直接是有效的 JSON
	var selectedTools map[string][]string
	if err := json.Unmarshal([]byte(response), &selectedTools); err != nil {
		g.Log().Errorf(ctx, "JSON解析失败: %v, 原始内容: %s", err, response)
		return nil, fmt.Errorf("JSON解析失败: %w", err)
	}

	return h.validateAndLimitTools(ctx, selectedTools), nil
}

// validateAndLimitTools 验证并限制工具数量
func (h *ChatHandler) validateAndLimitTools(ctx context.Context, selectedTools map[string][]string) map[string][]string {
	if selectedTools == nil {
		return nil
	}

	// 限制每个服务最多5个工具
	for serviceName, tools := range selectedTools {
		if len(tools) > 5 {
			selectedTools[serviceName] = tools[:5]
			g.Log().Warningf(ctx, "服务 %s 的工具数量超过5个，已截断为前5个", serviceName)
		}
		// 移除空工具列表的服务
		if len(tools) == 0 {
			delete(selectedTools, serviceName)
		}
	}

	return selectedTools
}
