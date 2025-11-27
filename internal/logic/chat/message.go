package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/core/common"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

const (
	role = "你是一个专业的AI助手，能够根据提供的参考信息准确回答用户问题。如果没有提供参考信息，也请根据你的知识自由回答用户问题。"
)

// formatDocuments 格式化文档列表为包含元数据的字符串
func formatDocuments(docs []*schema.Document) string {
	if len(docs) == 0 {
		// 当没有检索到相关文档时，返回空字符串，让大模型自由回答
		return ""
	}

	var builder strings.Builder
	builder.WriteString("\n")

	for i, doc := range docs {
		builder.WriteString(fmt.Sprintf("【参考资料 %d】\n", i+1))

		// 添加元数据信息
		if doc.MetaData != nil {
			// 从顶层获取document_id
			if docID, ok := doc.MetaData["document_id"]; ok {
				builder.WriteString(fmt.Sprintf("文档ID: %v\n", docID))
			}

			// 处理嵌套的metadata字段，从里面提取_source、_knowledge_id等
			if metadata, ok := doc.MetaData["metadata"]; ok {
				if metaMap, isMap := metadata.(map[string]interface{}); isMap {
					// 优先提取_source
					if source, ok := metaMap["_source"]; ok {
						builder.WriteString(fmt.Sprintf("来源: %v\n", source))
					}
					// 提取_knowledge_id
					if knowledgeID, ok := metaMap["knowledge_id"]; ok {
						builder.WriteString(fmt.Sprintf("知识库ID: %v\n", knowledgeID))
					}
					// 遍历其他字段
					for key, value := range metaMap {
						// 跳过已经处理的字段和content字段
						if key != "_source" && key != "_knowledge_id" && value != nil {
							builder.WriteString(fmt.Sprintf("%s: %v\n", key, value))
						}
					}
				}
			}

			// 处理聊天元数据
			if chatMetadata, ok := doc.MetaData["chat_metadata"]; ok {
				if metaMap, isMap := chatMetadata.(map[string]interface{}); isMap {
					builder.WriteString("聊天元数据:\n")
					for key, value := range metaMap {
						builder.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
					}
				}
			}
		}

		builder.WriteString("内容: ")
		builder.WriteString(doc.Content)
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// createTemplate 创建并返回一个配置好的聊天模板
func createTemplate() prompt.ChatTemplate {
	return prompt.FromMessages(schema.FString,
		// 系统消息模板
		schema.SystemMessage("{role}"+
			"你是一个智能助手，具备以下两种能力，请根据问题性质合理选择：\n\n"+
			"🔹 **知识库检索（RAG）**：\n"+
			"- 当前已为你提供相关参考内容（见下方「参考内容」）。\n"+
			"- 如果问题能从参考内容中直接或间接回答，请优先基于这些内容作答。\n"+
			"- 若参考内容不完整，可合理推断但需说明；若完全无关，请根据你的知识自由回答用户问题。\n\n"+
			"🔹 **工具调用（MCP）**：\n"+
			"- 对于需要实时数据、外部操作或动态计算的问题（如天气、时间、代码执行、数据库查询等），你可以调用可用工具。\n"+
			"- 工具列表及参数说明将由系统自动提供，你只需决定是否调用及传入正确参数。\n"+
			"- 不要虚构工具结果，也不要假设工具返回内容。\n\n"+
			"📌 回答要求：\n"+
			"- 保持专业、简洁、准确；\n"+
			"- 若使用了参考内容，可适当引用关键信息；\n"+
			"- 若调用了工具，请等待工具返回后再生成最终答案。\n\n"+
			"{formatted_docs}"), // 移除了"当前提供的参考内容："前缀，因为没有文档时应该完全不显示

		// 聊天历史（包含之前的 tool_call 和 tool 响应）
		schema.MessagesPlaceholder("chat_history", true),

		// 用户当前问题
		schema.UserMessage("Question: {question}"),
	)
}

// formatMessages 格式化消息并处理错误
func formatMessages(template prompt.ChatTemplate, data map[string]any) ([]*schema.Message, error) {
	messages, err := template.Format(context.Background(), data)
	if err != nil {
		return nil, fmt.Errorf("格式化模板失败: %w", err)
	}
	return messages, nil
}

// docsMessages 将检索到的上下文和问题转换为消息列表
func (x *Chat) docsMessages(ctx context.Context, convID string, docs []*schema.Document, question string) (messages []*schema.Message, err error) {
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return
	}
	// 插入一条用户数据
	err = x.eh.SaveMessage(&schema.Message{
		Role:    schema.User,
		Content: question,
	}, convID)
	if err != nil {
		return
	}
	template := createTemplate()
	for i, doc := range docs {
		g.Log().Debugf(context.Background(), "docs[%d]: %s", i, doc.Content)
	}

	// 格式化文档为包含元数据的字符串
	formattedDocs := formatDocuments(docs)
	g.Log().Debugf(context.Background(), "formatted docs: %s", formattedDocs)

	data := map[string]any{
		"role":           role,
		"question":       question,
		"formatted_docs": formattedDocs,
		"chat_history":   chatHistory,
	}
	messages, err = formatMessages(template, data)
	if err != nil {
		return
	}
	return
}

// docsMessagesWithFiles 将检索到的上下文、问题和文件转换为多模态消息列表
func (x *Chat) docsMessagesWithFiles(ctx context.Context, convID string, docs []*schema.Document, question string, files []*common.MultimodalFile) (messages []*schema.Message, err error) {
	chatHistory, err := x.eh.GetHistory(convID, 100)
	if err != nil {
		return
	}

	// 构建多模态消息
	multimodalBuilder := common.NewMultimodalMessageBuilder()

	// 使用base64编码方式（根据实际需求可以改为false使用URL方式）
	userMessage, err := multimodalBuilder.BuildMultimodalMessage(question, files, true)
	if err != nil {
		return nil, fmt.Errorf("构建多模态消息失败: %w", err)
	}

	// 插入用户消息
	err = x.eh.SaveMessage(userMessage, convID)
	if err != nil {
		return
	}

	template := createTemplate()
	for i, doc := range docs {
		g.Log().Debugf(context.Background(), "docs[%d]: %s", i, doc.Content)
	}

	// 格式化文档为包含元数据的字符串
	formattedDocs := formatDocuments(docs)
	g.Log().Debugf(context.Background(), "formatted docs: %s", formattedDocs)

	data := map[string]any{
		"role":           role,
		"question":       userMessage.Content, // 使用处理后的消息内容
		"formatted_docs": formattedDocs,
		"chat_history":   chatHistory,
	}

	// 生成系统消息和历史消息
	messages, err = formatMessages(template, data)
	if err != nil {
		return
	}

	// 如果有多模态内容，需要特殊处理最后一条用户消息
	// 将多模态信息添加到消息的Extra字段中
	if userMessage.Extra != nil {
		if multimodalContents, ok := userMessage.Extra["multimodal_contents"]; ok {
			// 找到最后一条用户消息并添加多模态内容
			for i := len(messages) - 1; i >= 0; i-- {
				if messages[i].Role == schema.User {
					if messages[i].Extra == nil {
						messages[i].Extra = make(map[string]any)
					}
					messages[i].Extra["multimodal_contents"] = multimodalContents
					break
				}
			}
		}
	}

	return
}
