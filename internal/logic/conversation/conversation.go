package conversation

import (
	"context"

	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/formatter"

	coreModel "github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/history"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// Manager 会话管理器
type Manager struct {
	historyManager *history.Manager
}

// NewManager 创建会话管理器
func NewManager() *Manager {
	return &Manager{
		historyManager: history.NewManager(),
	}
}

// ListConversations 获取会话列表
func (m *Manager) ListConversations(ctx context.Context, filters map[string]interface{}, page, pageSize int, sortBy, order string) ([]*ConversationItem, int64, error) {
	// 查询会话
	conversations, total, err := dao.Conversation.List(ctx, filters, page, pageSize, sortBy, order)
	if err != nil {
		return nil, 0, err
	}

	// 转换为响应格式
	items := make([]*ConversationItem, 0, len(conversations))
	for _, conv := range conversations {
		item, err := m.toConversationItem(ctx, conv)
		if err != nil {
			g.Log().Warningf(ctx, "转换会话项失败: %v", err)
			continue
		}
		items = append(items, item)
	}

	return items, total, nil
}

// GetConversationDetail 获取会话详情
func (m *Manager) GetConversationDetail(ctx context.Context, convID string) (*ConversationDetail, error) {
	// 查询会话
	conv, err := dao.Conversation.GetByConvID(ctx, convID)
	if err != nil {
		return nil, err
	}
	if conv == nil {
		return nil, errors.Newf(errors.ErrConversationNotFound, "会话不存在: %s", convID)
	}

	// 查询消息列表
	messages, err := m.historyManager.GetHistory(convID, 1000) // 最多返回1000条消息
	if err != nil {
		g.Log().Warningf(ctx, "查询会话消息失败: %v", err)
		messages = []*schema.Message{} // 返回空列表而不是报错
	}

	// 解析元数据
	var metadata map[string]any
	if len(conv.Metadata) > 0 {
		if err := json.Unmarshal(conv.Metadata, &metadata); err != nil {
			g.Log().Warningf(ctx, "解析会话元数据失败: %v", err)
		}
	}

	// 提取标签
	var tags []string
	if tagsData, ok := metadata["tags"].([]interface{}); ok {
		for _, tag := range tagsData {
			if tagStr, ok := tag.(string); ok {
				tags = append(tags, tagStr)
			}
		}
	}

	// 转换消息格式
	messageItems := make([]*MessageItem, 0, len(messages))
	for _, msg := range messages {
		createTime := time.Now().Format(time.RFC3339) // 默认使用当前时间

		// 从Extra字段中获取实际的创建时间
		if msg.Extra != nil {
			if timeStr, ok := msg.Extra["create_time"].(string); ok {
				createTime = timeStr
			}
		}

		messageItems = append(messageItems, &MessageItem{
			Role:             string(msg.Role),
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
			CreateTime:       createTime,
		})
	}

	return &ConversationDetail{
		ConvID:           conv.ConvID,
		UserID:           conv.UserID,
		Title:            conv.Title,
		ModelName:        conv.ModelName,
		ConversationType: conv.ConversationType,
		Status:           conv.Status,
		MessageCount:     len(messages),
		Messages:         messageItems,
		CreateTime:       conv.CreateTime.Format(time.RFC3339),
		UpdateTime:       conv.UpdateTime.Format(time.RFC3339),
		Tags:             tags,
		Metadata:         metadata,
	}, nil
}

// UpdateConversation 更新会话
func (m *Manager) UpdateConversation(ctx context.Context, convID string, title, status string, tags []string, metadata map[string]any) error {
	// 查询会话
	conv, err := dao.Conversation.GetByConvID(ctx, convID)
	if err != nil {
		return err
	}
	if conv == nil {
		return errors.Newf(errors.ErrConversationNotFound, "会话不存在: %s", convID)
	}

	// 准备更新字段
	updates := make(map[string]interface{})

	if title != "" {
		updates["title"] = title
	}

	if status != "" {
		updates["status"] = status
	}

	// 处理元数据和标签
	if tags != nil || metadata != nil {
		// 解析现有元数据
		var existingMetadata map[string]any
		if len(conv.Metadata) > 0 {
			if err := json.Unmarshal(conv.Metadata, &existingMetadata); err != nil {
				existingMetadata = make(map[string]any)
			}
		} else {
			existingMetadata = make(map[string]any)
		}

		// 更新标签
		if tags != nil {
			existingMetadata["tags"] = tags
		}

		// 合并元数据
		if metadata != nil {
			for k, v := range metadata {
				existingMetadata[k] = v
			}
		}

		// 序列化元数据
		metadataJSON, err := json.Marshal(existingMetadata)
		if err != nil {
			return errors.Newf(errors.ErrInternalError, "序列化元数据失败: %v", err)
		}
		updates["metadata"] = metadataJSON
	}

	// 更新时间
	now := time.Now()
	updates["update_time"] = &now

	// 执行更新
	return dao.Conversation.UpdateFields(ctx, convID, updates)
}

// DeleteConversation 删除会话
func (m *Manager) DeleteConversation(ctx context.Context, convID string) error {
	// 删除会话记录
	if err := dao.Conversation.Delete(ctx, convID); err != nil {
		return err
	}

	// 删除关联的消息历史
	if err := m.historyManager.DeleteConversationHistory(ctx, convID); err != nil {
		g.Log().Warningf(ctx, "删除消息历史失败: %v，但会话已删除", err)
		// 不返回错误，因为会话已经删除
	}

	return nil
}

// BatchDeleteConversations 批量删除会话
func (m *Manager) BatchDeleteConversations(ctx context.Context, convIDs []string) (int, []string, error) {
	failed := make([]string, 0)

	for _, convID := range convIDs {
		if err := m.DeleteConversation(ctx, convID); err != nil {
			g.Log().Errorf(ctx, "删除会话 %s 失败: %v", convID, err)
			failed = append(failed, convID)
		}
	}

	deletedCount := len(convIDs) - len(failed)
	return deletedCount, failed, nil
}

// GenerateSummary 生成会话摘要
func (m *Manager) GenerateSummary(ctx context.Context, convID, modelID, length string) (string, error) {
	// 查询会话消息
	historyMessages, err := m.historyManager.GetHistory(convID, 100) // 最多取100条消息
	if err != nil {
		return "", errors.Newf(errors.ErrDatabaseQuery, "查询会话消息失败: %v", err)
	}

	if len(historyMessages) == 0 {
		return "空会话", nil
	}

	// 构建摘要提示词
	var conversationText strings.Builder
	conversationText.WriteString("以下是一段对话内容，请生成摘要：\n\n")
	for _, msg := range historyMessages {
		conversationText.WriteString(fmt.Sprintf("%s: %s\n", msg.Role, msg.Content))
	}

	// 根据长度调整提示词
	lengthPrompt := ""
	switch length {
	case "short":
		lengthPrompt = "请用一句话（不超过30字）总结这段对话的核心主题。"
	case "long":
		lengthPrompt = "请详细总结这段对话，包括主要讨论点、关键结论和重要细节（100-200字）。"
	default: // medium
		lengthPrompt = "请简要总结这段对话的主要内容和关键点（50-100字）。"
	}
	conversationText.WriteString("\n")
	conversationText.WriteString(lengthPrompt)

	// 调用LLM生成摘要
	mc := coreModel.Registry.Get(modelID)
	if mc == nil {
		return "", errors.Newf(errors.ErrModelNotFound, "模型不存在: %s", modelID)
	}

	// 检查模型是否启用
	if !mc.Enabled {
		return "该模型已被禁用", nil
	}

	// 创建消息格式化器和模型服务
	msgFormatter := formatter.NewOpenAIFormatter()
	modelService := coreModel.NewModelService(mc.APIKey, mc.BaseURL, msgFormatter)

	// 构造消息
	messages := []*schema.Message{
		{
			Role:    schema.User,
			Content: conversationText.String(),
		},
	}

	// 调用模型生成摘要
	resp, err := modelService.ChatCompletion(ctx, coreModel.ChatCompletionParams{
		ModelName:           mc.Name,
		Messages:            messages,
		Temperature:         0.3, // 较低温度以获得更稳定的摘要
		MaxCompletionTokens: 200, // 限制摘要长度
	})

	if err != nil {
		g.Log().Warningf(ctx, "调用模型生成摘要失败: %v，使用默认摘要", err)
		return fmt.Sprintf("包含 %d 条消息的对话", len(historyMessages)), nil
	}

	if len(resp.Choices) == 0 {
		g.Log().Warningf(ctx, "模型返回为空，使用默认摘要")
		return fmt.Sprintf("包含 %d 条消息的对话", len(historyMessages)), nil
	}

	summary := strings.TrimSpace(resp.Choices[0].Message.Content)
	if summary == "" {
		summary = fmt.Sprintf("包含 %d 条消息的对话", len(historyMessages))
	}

	return summary, nil
}

// ExportConversation 导出会话
func (m *Manager) ExportConversation(ctx context.Context, convID, format string) (string, string, error) {
	// 获取会话详情
	detail, err := m.GetConversationDetail(ctx, convID)
	if err != nil {
		return "", "", err
	}

	filename := fmt.Sprintf("conversation_%s_%s.%s", convID, time.Now().Format("20060102150405"), format)

	switch format {
	case "json":
		data, err := json.MarshalIndent(detail, "", "  ")
		if err != nil {
			return "", "", errors.Newf(errors.ErrInternalError, "JSON序列化失败: %v", err)
		}
		return string(data), filename, nil

	case "markdown":
		var md strings.Builder
		md.WriteString(fmt.Sprintf("# %s\n\n", detail.Title))
		md.WriteString(fmt.Sprintf("- **会话ID**: %s\n", detail.ConvID))
		md.WriteString(fmt.Sprintf("- **模型**: %s\n", detail.ModelName))
		md.WriteString(fmt.Sprintf("- **创建时间**: %s\n", detail.CreateTime))
		md.WriteString(fmt.Sprintf("- **消息数**: %d\n\n", detail.MessageCount))

		md.WriteString("## 对话内容\n\n")
		for _, msg := range detail.Messages {
			md.WriteString(fmt.Sprintf("### %s\n\n", msg.Role))
			md.WriteString(fmt.Sprintf("%s\n\n", msg.Content))
			if msg.ReasoningContent != "" {
				md.WriteString(fmt.Sprintf("**思考过程**:\n%s\n\n", msg.ReasoningContent))
			}
			md.WriteString("---\n\n")
		}
		return md.String(), filename, nil

	case "txt":
		var txt strings.Builder
		txt.WriteString(fmt.Sprintf("会话: %s\n", detail.Title))
		txt.WriteString(fmt.Sprintf("会话ID: %s\n", detail.ConvID))
		txt.WriteString(fmt.Sprintf("模型: %s\n", detail.ModelName))
		txt.WriteString(fmt.Sprintf("创建时间: %s\n", detail.CreateTime))
		txt.WriteString(fmt.Sprintf("消息数: %d\n\n", detail.MessageCount))
		txt.WriteString("========================================\n\n")

		for _, msg := range detail.Messages {
			txt.WriteString(fmt.Sprintf("[%s]\n%s\n\n", msg.Role, msg.Content))
		}
		return txt.String(), filename, nil

	default:
		return "", "", errors.Newf(errors.ErrInvalidParameter, "不支持的导出格式: %s", format)
	}
}

// toConversationItem 转换为会话列表项
func (m *Manager) toConversationItem(ctx context.Context, conv *gormModel.Conversation) (*ConversationItem, error) {
	// 查询消息数量
	messages, err := m.historyManager.GetHistory(conv.ConvID, 1)
	if err != nil {
		g.Log().Warningf(ctx, "查询会话消息失败: %v", err)
	}

	messageCount := len(messages)
	lastMessage := ""
	lastMessageTime := conv.UpdateTime.Format(time.RFC3339)

	if len(messages) > 0 {
		lastMessage = messages[0].Content
		if len(lastMessage) > 100 {
			lastMessage = lastMessage[:100] + "..."
		}
	}

	// 解析元数据
	var metadata map[string]any
	var tags []string
	if len(conv.Metadata) > 0 {
		if err := json.Unmarshal(conv.Metadata, &metadata); err != nil {
			g.Log().Warningf(ctx, "解析元数据失败: %v", err)
		} else {
			if tagsData, ok := metadata["tags"].([]interface{}); ok {
				for _, tag := range tagsData {
					if tagStr, ok := tag.(string); ok {
						tags = append(tags, tagStr)
					}
				}
			}
		}
	}

	return &ConversationItem{
		ConvID:           conv.ConvID,
		Title:            conv.Title,
		ModelName:        conv.ModelName,
		ConversationType: conv.ConversationType,
		Status:           conv.Status,
		MessageCount:     messageCount,
		LastMessage:      lastMessage,
		LastMessageTime:  lastMessageTime,
		CreateTime:       conv.CreateTime.Format(time.RFC3339),
		UpdateTime:       conv.UpdateTime.Format(time.RFC3339),
		Tags:             tags,
		Metadata:         metadata,
	}, nil
}

// ConversationItem 会话列表项
type ConversationItem struct {
	ConvID           string         `json:"conv_id"`
	Title            string         `json:"title"`
	ModelName        string         `json:"model_name"`
	ConversationType string         `json:"conversation_type"`
	Status           string         `json:"status"`
	MessageCount     int            `json:"message_count"`
	LastMessage      string         `json:"last_message"`
	LastMessageTime  string         `json:"last_message_time"`
	CreateTime       string         `json:"create_time"`
	UpdateTime       string         `json:"update_time"`
	Tags             []string       `json:"tags,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// ConversationDetail 会话详情
type ConversationDetail struct {
	ConvID           string         `json:"conv_id"`
	UserID           string         `json:"user_id"`
	Title            string         `json:"title"`
	ModelName        string         `json:"model_name"`
	ConversationType string         `json:"conversation_type"`
	Status           string         `json:"status"`
	MessageCount     int            `json:"message_count"`
	Messages         []*MessageItem `json:"messages"`
	CreateTime       string         `json:"create_time"`
	UpdateTime       string         `json:"update_time"`
	Tags             []string       `json:"tags,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// MessageItem 消息项
type MessageItem struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
	CreateTime       string `json:"create_time"`
	TokensUsed       int    `json:"tokens_used,omitempty"`
	LatencyMs        int    `json:"latency_ms,omitempty"`
}
