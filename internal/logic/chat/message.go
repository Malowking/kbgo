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
			if source, ok := doc.MetaData["source"]; ok {
				builder.WriteString(fmt.Sprintf("来源: %v\n", source))
			}
			if page, ok := doc.MetaData["page"]; ok {
				builder.WriteString(fmt.Sprintf("页码: %v\n", page))
			}
			if docID, ok := doc.MetaData["document_id"]; ok {
				builder.WriteString(fmt.Sprintf("文档ID: %v\n", docID))
			}
			if title, ok := doc.MetaData["title"]; ok {
				builder.WriteString(fmt.Sprintf("标题: %v\n", title))
			}
			// 处理嵌套的metadata字段（常见情况）
			if metadata, ok := doc.MetaData["metadata"]; ok {
				if metaMap, isMap := metadata.(map[string]interface{}); isMap {
					for key, value := range metaMap {
						if key != "content" && value != nil {
							builder.WriteString(fmt.Sprintf("%s: %v\n", key, value))
						}
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
	// 创建模板，使用 FString 格式
	return prompt.FromMessages(schema.FString,
		// 系统消息模板
		schema.SystemMessage("{role}"+
			"请严格遵守以下规则：\n"+
			"1. 回答必须基于提供的参考内容，不要依赖外部知识\n"+
			"2. 如果参考内容中有明确答案，直接使用参考内容回答\n"+
			"3. 如果参考内容不完整或模糊，可以合理推断但需说明\n"+
			"4. 如果参考内容完全不相关或不存在，如实告知用户'根据现有资料无法回答'\n"+
			"5. 保持回答专业、简洁、准确\n"+
			"6. 必要时可引用参考内容中的具体数据或原文\n\n"+
			"当前提供的参考内容：{formatted_docs}\n"+
			""),
		schema.MessagesPlaceholder("chat_history", true),
		// 用户消息模板
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
