package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/schema"
)

const (
	role = "你是一个专业的AI助手，能够根据提供的参考信息准确回答用户问题。如果没有提供参考信息，也请根据你的知识自由回答用户问题。"
)

const systemPromptTemplate = `%s
你是一个智能助手，具备以下两种能力，请根据问题性质合理选择：

🔹 **知识库检索（RAG）**：
- 当前已为你提供相关参考内容（见下方「参考内容」）。
- 如果问题能从参考内容中直接或间接回答，请优先基于这些内容作答。
- 若参考内容不完整，可合理推断但需说明；若完全无关，请根据你的知识自由回答用户问题。

🔹 **工具调用（MCP）**：
- 对于需要实时数据、外部操作或动态计算的问题（如天气、时间、代码执行、数据库查询等），你可以调用可用工具。
- 工具列表及参数说明将由系统自动提供，你只需决定是否调用及传入正确参数。
- 不要虚构工具结果，也不要假设工具返回内容。

📌 回答要求：
- 保持专业、简洁、准确；
- 若使用了参考内容，可适当引用关键信息；
- 若调用了工具，请等待工具返回后再生成最终答案。

%s`

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

// buildSystemMessage 构建系统消息
func buildSystemMessage(formattedDocs string) string {
	return fmt.Sprintf(systemPromptTemplate, role, formattedDocs)
}

// docsMessages 将检索到的上下文和问题转换为消息列表
func (x *Chat) docsMessages(ctx context.Context, convID string, docs []*schema.Document, question string) (messages []*schema.Message, err error) {
	chatHistory, err := x.eh.GetHistory(convID, 50)
	if err != nil {
		return
	}

	// 捕获用户消息接收时间
	userMessageTime := time.Now()

	err = x.eh.SaveMessage(&schema.Message{
		Role:    schema.User,
		Content: question,
	}, convID, nil, &userMessageTime)
	if err != nil {
		return
	}

	// 格式化文档为包含元数据的字符串
	formattedDocs := formatDocuments(docs)

	// 构建系统消息
	systemContent := buildSystemMessage(formattedDocs)
	messages = []*schema.Message{
		{
			Role:    schema.System,
			Content: systemContent,
		},
	}

	// 添加聊天历史
	messages = append(messages, chatHistory...)

	// 添加用户当前问题
	messages = append(messages, &schema.Message{
		Role:    schema.User,
		Content: fmt.Sprintf("Question: %s", question),
	})

	return
}
