package dao

import (
	"context"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// ConversationDAO 会话数据访问对象
type ConversationDAO struct{}

var Conversation = &ConversationDAO{}

// Create 创建会话
func (d *ConversationDAO) Create(ctx context.Context, conversation *gormModel.Conversation) error {
	if err := GetDB().WithContext(ctx).Create(conversation).Error; err != nil {
		g.Log().Errorf(ctx, "创建会话失败: %v", err)
		return err
	}
	return nil
}

// GetByConvID 根据会话ID获取会话
func (d *ConversationDAO) GetByConvID(ctx context.Context, convID string) (*gormModel.Conversation, error) {
	var conversation gormModel.Conversation
	if err := GetDB().WithContext(ctx).Where("conv_id = ?", convID).First(&conversation).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		g.Log().Errorf(ctx, "查询会话失败: %v", err)
		return nil, err
	}
	return &conversation, nil
}

// ListByUserID 根据用户ID获取会话列表
func (d *ConversationDAO) ListByUserID(ctx context.Context, userID string, page, pageSize int) ([]*gormModel.Conversation, int64, error) {
	var conversations []*gormModel.Conversation
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.Conversation{}).Where("user_id = ?", userID)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "统计会话总数失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("update_time DESC").Find(&conversations).Error; err != nil {
		g.Log().Errorf(ctx, "查询会话列表失败: %v", err)
		return nil, 0, err
	}

	return conversations, total, nil
}

// Update 更新会话
func (d *ConversationDAO) Update(ctx context.Context, conversation *gormModel.Conversation) error {
	if err := GetDB().WithContext(ctx).Save(conversation).Error; err != nil {
		g.Log().Errorf(ctx, "更新会话失败: %v", err)
		return err
	}
	return nil
}

// Delete 删除会话
func (d *ConversationDAO) Delete(ctx context.Context, convID string) error {
	if err := GetDB().WithContext(ctx).Where("conv_id = ?", convID).Delete(&gormModel.Conversation{}).Error; err != nil {
		g.Log().Errorf(ctx, "删除会话失败: %v", err)
		return err
	}
	return nil
}

// UpdateMetadata 更新会话元数据
func (d *ConversationDAO) UpdateMetadata(ctx context.Context, convID string, metadata gormModel.JSON) error {
	if err := GetDB().WithContext(ctx).Model(&gormModel.Conversation{}).Where("conv_id = ?", convID).Update("metadata", metadata).Error; err != nil {
		g.Log().Errorf(ctx, "更新会话元数据失败: %v", err)
		return err
	}
	return nil
}
