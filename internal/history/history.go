package history

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MessageWithContents 带内容块的消息结构
type MessageWithContents struct {
	*gormModel.Message // 包含 msg_id, role, tool_calls 等
	Contents           []gormModel.MessageContent
}

// MessageWithMetrics 带指标的消息结构
type MessageWithMetrics struct {
	*schema.Message
	TokensUsed int
	LatencyMs  int
	TraceID    string
	ToolCalls  []*schema.ToolCall
}

// Manager 聊天历史管理器
type Manager struct {
	db *gorm.DB
}

// NewManager 创建新的聊天历史管理器
func NewManager() *Manager {
	return &Manager{
		db: dao.GetDB(),
	}
}

// SaveMessage 保存消息
func (h *Manager) SaveMessage(message *schema.Message, convID string) error {
	return h.SaveMessageWithMetadata(message, convID, nil)
}

// SaveMessageWithMetrics 保存带指标的消息
func (h *Manager) SaveMessageWithMetrics(message *MessageWithMetrics, convID string) error {
	// 确保对话存在
	if err := h.ensureConversationExists(convID); err != nil {
		return err
	}

	now := time.Now()

	// 处理工具调用
	var toolCallsJSON gormModel.JSON
	if message.ToolCalls != nil {
		data, err := json.Marshal(message.ToolCalls)
		if err != nil {
			return fmt.Errorf("failed to marshal tool calls: %w", err)
		}
		toolCallsJSON = gormModel.JSON(data)
	}

	// 创建消息记录
	msg := &gormModel.Message{
		MsgID:      generateMessageID(),
		ConvID:     convID,
		Role:       string(message.Role),
		CreateTime: &now,
		TokensUsed: message.TokensUsed,
		LatencyMs:  message.LatencyMs,
		TraceID:    message.TraceID,
		ToolCalls:  toolCallsJSON,
	}

	// 处理内容块
	var contents []*gormModel.MessageContent
	content := &gormModel.MessageContent{
		ContentType: "text",
		TextContent: message.Content,
		SortOrder:   0,
		CreateTime:  &now,
	}
	contents = append(contents, content)

	return dao.Message.CreateWithContents(nil, msg, contents)
}

// SaveMessageWithMetadata 保存带元数据的消息
func (h *Manager) SaveMessageWithMetadata(message *schema.Message, convID string, metadata map[string]interface{}) error {
	// 确保对话存在
	if err := h.ensureConversationExists(convID); err != nil {
		return err
	}

	// 处理元数据
	var metadataJSON gormModel.JSON
	if metadata != nil {
		data, err := json.Marshal(metadata)
		if err != nil {
			return fmt.Errorf("failed to marshal metadata: %w", err)
		}
		metadataJSON = gormModel.JSON(data)
	}

	now := time.Now()
	// 创建消息记录
	msg := &gormModel.Message{
		MsgID:      generateMessageID(),
		ConvID:     convID,
		Role:       string(message.Role),
		CreateTime: &now,
		Metadata:   metadataJSON,
	}

	// 处理内容块
	var contents []*gormModel.MessageContent
	content := &gormModel.MessageContent{
		ContentType: "text",
		TextContent: message.Content,
		SortOrder:   0,
		CreateTime:  &now,
	}
	contents = append(contents, content)

	return dao.Message.CreateWithContents(nil, msg, contents)
}

// GetHistory 获取聊天历史
func (h *Manager) GetHistory(convID string, limit int) ([]*schema.Message, error) {
	if limit <= 0 {
		limit = 100
	}

	// 获取消息列表
	messages, _, err := dao.Message.ListByConvID(nil, convID, 1, limit)
	if err != nil {
		return nil, err
	}

	// 获取所有消息ID
	var msgIDs []string
	for _, msg := range messages {
		msgIDs = append(msgIDs, msg.MsgID)
	}

	// 批量获取内容块
	contents, err := dao.MessageContent.ListByMsgIDs(nil, msgIDs)
	if err != nil {
		return nil, err
	}

	// 按消息ID组织内容块
	contentMap := make(map[string][]*gormModel.MessageContent)
	for _, content := range contents {
		contentMap[content.MsgID] = append(contentMap[content.MsgID], content)
	}

	// 转换为 schema.Message
	result := make([]*schema.Message, len(messages))
	for i, msg := range messages {
		// 获取该消息的内容块
		msgContents := contentMap[msg.MsgID]

		// 构建消息内容（这里简化处理，只取第一个文本内容）
		content := ""
		if len(msgContents) > 0 {
			content = msgContents[0].TextContent
		}

		result[i] = &schema.Message{
			Role:    schema.RoleType(msg.Role),
			Content: content,
		}
	}

	return result, nil
}

// GetConversationHistory 获取会话历史消息
func (h *Manager) GetConversationHistory(convID string) ([]MessageWithContents, error) {
	var msgs []MessageWithContents
	err := dao.GetDB().
		Where("conv_id = ?", convID).
		Order("create_time ASC").
		Find(&msgs).
		Error

	if err != nil {
		return nil, err
	}

	// 预加载消息内容
	for i := range msgs {
		var contents []gormModel.MessageContent
		err := dao.GetDB().
			Where("msg_id = ?", msgs[i].MsgID).
			Order("sort_order ASC").
			Find(&contents).
			Error

		if err != nil {
			return nil, err
		}

		msgs[i].Contents = contents
	}

	return msgs, nil
}

// BuildLLMMessages 将历史消息转换为LLM格式
func (h *Manager) BuildLLMMessages(history []MessageWithContents) []map[string]interface{} {
	var llmMsgs []map[string]interface{}

	for _, m := range history {
		// 处理 tool 消息
		if m.Role == "tool" {
			// tool 消息：content 是 JSON 字符串，需解析为文本描述或保留原样
			content := fmt.Sprintf("[Tool Result: %s]", m.Contents[0].TextContent)
			llmMsgs = append(llmMsgs, map[string]interface{}{
				"role":         "tool",
				"content":      content,
				"tool_call_id": m.ToolCallID,
			})
			continue
		}

		// 合并 message_contents 为纯文本
		var parts []string
		for _, c := range m.Contents {
			switch c.ContentType {
			case "text", "json_data", "tool_result":
				parts = append(parts, c.TextContent)
			case "image_url", "audio_url", "file_url":
				// 当前仅 LLM，无法理解媒体，转为文本描述
				parts = append(parts, fmt.Sprintf("[Uploaded file: %s]", extractFileName(c.MediaURL)))
			case "file_binary_ref":
				parts = append(parts, fmt.Sprintf("[File uploaded: %s]", c.StorageKey))
			}
		}
		content := strings.Join(parts, "\n")

		// 处理 assistant 的 tool_calls
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			// OpenAI 格式：content 可为空，tool_calls 单独字段
			msg := map[string]interface{}{
				"role":       "assistant",
				"content":    content,
				"tool_calls": m.ToolCalls, // 假设 ToolCalls 是 json.RawMessage 或 map
			}
			llmMsgs = append(llmMsgs, msg)
		} else {
			llmMsgs = append(llmMsgs, map[string]interface{}{
				"role":    m.Role,
				"content": content,
			})
		}
	}

	return llmMsgs
}

// TruncateMessagesByToken 根据token数量截断消息
func (h *Manager) TruncateMessagesByToken(messages []map[string]interface{}, maxTokens int, model string) []map[string]interface{} {
	// 粗略估算：1 token 4 英文字符 or 1.5 中文字符
	// 更准的方式：使用 tiktoken 库（见下方建议）
	totalTokens := 0
	startIdx := 0

	for i, msg := range messages {
		content, _ := msg["content"].(string)
		tokens := h.estimateTokenCount(content) + 10 // + role 开销
		if totalTokens+tokens > maxTokens {
			startIdx = i
			break
		}
		totalTokens += tokens
	}

	// 保证至少包含最后一条用户消息
	if startIdx >= len(messages) {
		startIdx = len(messages) - 1
	}

	return messages[startIdx:]
}

// estimateTokenCount 估算token数量
func (h *Manager) estimateTokenCount(text string) int {
	// 简化版：中文按 1.5 字/词，英文按 4 字/词
	chinese := utf8.RuneCountInString(regexp.MustCompile(`[\p{Han}]`).ReplaceAllString(text, ""))
	english := len(regexp.MustCompile(`[a-zA-Z0-9]+`).ReplaceAllString(text, "")) / 4
	return chinese + english
}

// extractFileName 从URL中提取文件名
func extractFileName(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}

// ensureConversationExists 确保对话存在
func (h *Manager) ensureConversationExists(convID string) error {
	conversation, err := dao.Conversation.GetByConvID(nil, convID)
	if err != nil {
		return err
	}

	if conversation == nil {
		now := time.Now()
		conversation := &gormModel.Conversation{
			ConvID:           convID,
			UserID:           "default_user", // 默认用户ID，实际使用时应从上下文获取
			Title:            "New Conversation",
			ModelName:        "default_model", // 默认模型名
			ConversationType: "text",
			Status:           "active",
			CreateTime:       &now,
			UpdateTime:       &now,
		}
		return dao.Conversation.Create(nil, conversation)
	}

	return nil
}

// generateMessageID 生成消息ID
func generateMessageID() string {
	return uuid.New().String()
}

// GetMessageMetadata 获取消息的元数据
func (h *Manager) GetMessageMetadata(msgID string) (map[string]interface{}, error) {
	message, err := dao.Message.GetByMsgID(nil, msgID)
	if err != nil {
		return nil, err
	}

	if message == nil || len(message.Metadata) == 0 {
		return nil, nil
	}

	var metadata map[string]interface{}
	err = json.Unmarshal(message.Metadata, &metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return metadata, nil
}
