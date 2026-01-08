package kbgo

import (
	"context"
	"sync"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/logic/conversation"
	"github.com/gogf/gf/v2/frame/g"
)

var (
	conversationManager     *conversation.Manager
	conversationManagerOnce sync.Once
)

// getConversationManager 延迟初始化并获取会话管理器
func getConversationManager() *conversation.Manager {
	conversationManagerOnce.Do(func() {
		conversationManager = conversation.NewManager()
	})
	return conversationManager
}

// ConversationList 获取会话列表
func (c *ControllerV1) ConversationList(ctx context.Context, req *v1.ConversationListReq) (res *v1.ConversationListRes, err error) {
	g.Log().Infof(ctx, "ConversationList request - KnowledgeID: %s, ConversationType: %s, Page: %d, PageSize: %d", req.KnowledgeID, req.ConversationType, req.Page, req.PageSize)

	// 构建筛选条件
	filters := make(map[string]interface{})
	if req.KnowledgeID != "" {
		filters["knowledge_id"] = req.KnowledgeID
	}
	if req.ConversationType != "" {
		filters["conversation_type"] = req.ConversationType
	}
	if req.Status != "" {
		filters["status"] = req.Status
	}

	// 查询会话列表
	items, total, err := getConversationManager().ListConversations(ctx, filters, req.Page, req.PageSize, req.SortBy, req.Order)
	if err != nil {
		g.Log().Errorf(ctx, "查询会话列表失败: %v", err)
		return nil, err
	}

	// 转换响应格式
	conversations := make([]*v1.ConversationItem, 0, len(items))
	for _, item := range items {
		conversations = append(conversations, &v1.ConversationItem{
			ConvID:           item.ConvID,
			Title:            item.Title,
			ModelName:        item.ModelName,
			ConversationType: item.ConversationType,
			Status:           item.Status,
			MessageCount:     item.MessageCount,
			LastMessage:      item.LastMessage,
			LastMessageTime:  item.LastMessageTime,
			CreateTime:       item.CreateTime,
			UpdateTime:       item.UpdateTime,
			Tags:             item.Tags,
			Metadata:         item.Metadata,
		})
	}

	return &v1.ConversationListRes{
		Conversations: conversations,
		Total:         total,
		Page:          req.Page,
		PageSize:      req.PageSize,
	}, nil
}

// ConversationDetail 获取会话详情
func (c *ControllerV1) ConversationDetail(ctx context.Context, req *v1.ConversationDetailReq) (res *v1.ConversationDetailRes, err error) {
	g.Log().Infof(ctx, "ConversationDetail request - ConvID: %s", req.ConvID)

	detail, err := getConversationManager().GetConversationDetail(ctx, req.ConvID)
	if err != nil {
		g.Log().Errorf(ctx, "查询会话详情失败: %v", err)
		return nil, err
	}

	// 转换消息格式
	messages := make([]*v1.MessageItem, 0, len(detail.Messages))
	for _, msg := range detail.Messages {
		messages = append(messages, &v1.MessageItem{
			Role:             msg.Role,
			Content:          msg.Content,
			ReasoningContent: msg.ReasoningContent,
			CreateTime:       msg.CreateTime,
			TokensUsed:       msg.TokensUsed,
			LatencyMs:        msg.LatencyMs,
		})
	}

	return &v1.ConversationDetailRes{
		ConvID:           detail.ConvID,
		UserID:           detail.UserID,
		Title:            detail.Title,
		ModelName:        detail.ModelName,
		ConversationType: detail.ConversationType,
		Status:           detail.Status,
		MessageCount:     detail.MessageCount,
		Messages:         messages,
		CreateTime:       detail.CreateTime,
		UpdateTime:       detail.UpdateTime,
		Tags:             detail.Tags,
		Metadata:         detail.Metadata,
	}, nil
}

// ConversationDelete 删除会话
func (c *ControllerV1) ConversationDelete(ctx context.Context, req *v1.ConversationDeleteReq) (res *v1.ConversationDeleteRes, err error) {
	g.Log().Infof(ctx, "ConversationDelete request - ConvID: %s", req.ConvID)

	if err := getConversationManager().DeleteConversation(ctx, req.ConvID); err != nil {
		g.Log().Errorf(ctx, "删除会话失败: %v", err)
		return nil, err
	}

	return &v1.ConversationDeleteRes{
		Message: "会话删除成功",
	}, nil
}

// ConversationUpdate 更新会话
func (c *ControllerV1) ConversationUpdate(ctx context.Context, req *v1.ConversationUpdateReq) (res *v1.ConversationUpdateRes, err error) {
	g.Log().Infof(ctx, "ConversationUpdate request - ConvID: %s, Title: %s, Status: %s", req.ConvID, req.Title, req.Status)

	if err := getConversationManager().UpdateConversation(ctx, req.ConvID, req.Title, req.Status, req.Tags, req.Metadata); err != nil {
		g.Log().Errorf(ctx, "更新会话失败: %v", err)
		return nil, err
	}

	return &v1.ConversationUpdateRes{
		Message: "会话更新成功",
	}, nil
}

// ConversationSummary 生成会话摘要
func (c *ControllerV1) ConversationSummary(ctx context.Context, req *v1.ConversationSummaryReq) (res *v1.ConversationSummaryRes, err error) {
	g.Log().Infof(ctx, "ConversationSummary request - ConvID: %s, ModelID: %s, Length: %s", req.ConvID, req.ModelID, req.Length)

	summary, err := getConversationManager().GenerateSummary(ctx, req.ConvID, req.ModelID, req.Length)
	if err != nil {
		g.Log().Errorf(ctx, "生成会话摘要失败: %v", err)
		return nil, err
	}

	return &v1.ConversationSummaryRes{
		Summary: summary,
	}, nil
}

// ConversationExport 导出会话
func (c *ControllerV1) ConversationExport(ctx context.Context, req *v1.ConversationExportReq) (res *v1.ConversationExportRes, err error) {
	g.Log().Infof(ctx, "ConversationExport request - ConvID: %s, Format: %s", req.ConvID, req.Format)

	content, filename, err := getConversationManager().ExportConversation(ctx, req.ConvID, req.Format)
	if err != nil {
		g.Log().Errorf(ctx, "导出会话失败: %v", err)
		return nil, err
	}

	return &v1.ConversationExportRes{
		Content:  content,
		Filename: filename,
	}, nil
}

// ConversationBatchDelete 批量删除会话
func (c *ControllerV1) ConversationBatchDelete(ctx context.Context, req *v1.ConversationBatchDeleteReq) (res *v1.ConversationBatchDeleteRes, err error) {
	g.Log().Infof(ctx, "ConversationBatchDelete request - ConvIDs: %v", req.ConvIDs)

	deletedCount, failed, err := getConversationManager().BatchDeleteConversations(ctx, req.ConvIDs)
	if err != nil {
		g.Log().Errorf(ctx, "批量删除会话失败: %v", err)
		return nil, err
	}

	message := "批量删除成功"
	if len(failed) > 0 {
		message = "部分删除失败"
	}

	return &v1.ConversationBatchDeleteRes{
		DeletedCount: deletedCount,
		FailedConvs:  failed,
		Message:      message,
	}, nil
}

// CreateAgentConversation 创建Agent对话
func (c *ControllerV1) CreateAgentConversation(ctx context.Context, req *v1.CreateAgentConversationReq) (res *v1.CreateAgentConversationRes, err error) {
	g.Log().Infof(ctx, "CreateAgentConversation request - ConvID: %s, PresetID: %s, UserID: %s", req.ConvID, req.PresetID, req.UserID)

	// 创建会话记录
	if err := getConversationManager().CreateAgentConversation(ctx, req.ConvID, req.PresetID, req.UserID, req.Title); err != nil {
		g.Log().Errorf(ctx, "创建Agent对话失败: %v", err)
		return nil, err
	}

	return &v1.CreateAgentConversationRes{
		ConvID: req.ConvID,
	}, nil
}
