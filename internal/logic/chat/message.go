package chat

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/frame/g"
)

const (
	role = "你是一个专业的AI助手，能够根据提供的参考信息准确回答用户问题。"
)

// formatDocuments 格式化文档列表为包含元数据的字符串
func formatDocuments(docs []*schema.Document) string {
	if len(docs) == 0 {
		return "暂无相关参考资料"
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

			// 从顶层获取可能存在的其他字段
			//if page, ok := doc.MetaData["page"]; ok {
			//	builder.WriteString(fmt.Sprintf("页码: %v\n", page))
			//}
			//if title, ok := doc.MetaData["title"]; ok {
			//	builder.WriteString(fmt.Sprintf("标题: %v\n", title))
			//}

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
			"- 若参考内容不完整，可合理推断但需说明；若完全无关，请明确回复“根据现有资料无法回答”。\n\n"+
			"🔹 **工具调用（MCP）**：\n"+
			"- 对于需要实时数据、外部操作或动态计算的问题（如天气、时间、代码执行、数据库查询等），你可以调用可用工具。\n"+
			"- 工具列表及参数说明将由系统自动提供，你只需决定是否调用及传入正确参数。\n"+
			"- 不要虚构工具结果，也不要假设工具返回内容。\n\n"+
			"📌 回答要求：\n"+
			"- 保持专业、简洁、准确；\n"+
			"- 若使用了参考内容，可适当引用关键信息；\n"+
			"- 若调用了工具，请等待工具返回后再生成最终答案。\n\n"+
			"当前提供的参考内容：{formatted_docs}\n"+
			""),

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
