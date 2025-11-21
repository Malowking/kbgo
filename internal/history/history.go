package history

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

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
	return "msg_" + uuid.New().String()
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
