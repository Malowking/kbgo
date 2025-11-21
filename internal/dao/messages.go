package dao

import (
	"context"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// MessageDAO 消息数据访问对象
type MessageDAO struct{}

var Message = &MessageDAO{}

// Create 创建消息
func (d *MessageDAO) Create(ctx context.Context, message *gormModel.Message) error {
	if err := GetDB().WithContext(ctx).Create(message).Error; err != nil {
		g.Log().Errorf(ctx, "创建消息失败: %v", err)
		return err
	}
	return nil
}

// CreateWithContents 创建消息及内容块
func (d *MessageDAO) CreateWithContents(ctx context.Context, message *gormModel.Message, contents []*gormModel.MessageContent) error {
	return GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 创建消息
		if err := tx.Create(message).Error; err != nil {
			g.Log().Errorf(ctx, "创建消息失败: %v", err)
			return err
		}

		// 创建内容块
		for _, content := range contents {
			content.MsgID = message.MsgID
			if err := tx.Create(content).Error; err != nil {
				g.Log().Errorf(ctx, "创建消息内容块失败: %v", err)
				return err
			}
		}

		return nil
	})
}

// GetByMsgID 根据消息ID获取消息
func (d *MessageDAO) GetByMsgID(ctx context.Context, msgID string) (*gormModel.Message, error) {
	var message gormModel.Message
	if err := GetDB().WithContext(ctx).Where("msg_id = ?", msgID).First(&message).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		g.Log().Errorf(ctx, "查询消息失败: %v", err)
		return nil, err
	}
	return &message, nil
}

// ListByConvID 根据会话ID获取消息列表
func (d *MessageDAO) ListByConvID(ctx context.Context, convID string, page, pageSize int) ([]*gormModel.Message, int64, error) {
	var messages []*gormModel.Message
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.Message{}).Where("conv_id = ?", convID)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "统计消息总数失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("create_time ASC").Find(&messages).Error; err != nil {
		g.Log().Errorf(ctx, "查询消息列表失败: %v", err)
		return nil, 0, err
	}

	return messages, total, nil
}

// ListByConvIDWithContents 根据会话ID获取消息及内容块列表
func (d *MessageDAO) ListByConvIDWithContents(ctx context.Context, convID string) ([]*gormModel.Message, error) {
	var messages []*gormModel.Message

	// 查询消息
	if err := GetDB().WithContext(ctx).Where("conv_id = ?", convID).Order("create_time ASC").Find(&messages).Error; err != nil {
		g.Log().Errorf(ctx, "查询消息列表失败: %v", err)
		return nil, err
	}

	// 查询每个消息的内容块
	for _, message := range messages {
		var contents []*gormModel.MessageContent
		if err := GetDB().WithContext(ctx).Where("msg_id = ?", message.MsgID).Order("sort_order ASC").Find(&contents).Error; err != nil {
			g.Log().Errorf(ctx, "查询消息内容块失败: %v", err)
			return nil, err
		}
		// 这里需要在Message结构体中添加Contents字段才能关联
	}

	return messages, nil
}

// Update 更新消息
func (d *MessageDAO) Update(ctx context.Context, message *gormModel.Message) error {
	if err := GetDB().WithContext(ctx).Save(message).Error; err != nil {
		g.Log().Errorf(ctx, "更新消息失败: %v", err)
		return err
	}
	return nil
}

// Delete 删除消息
func (d *MessageDAO) Delete(ctx context.Context, msgID string) error {
	if err := GetDB().WithContext(ctx).Where("msg_id = ?", msgID).Delete(&gormModel.Message{}).Error; err != nil {
		g.Log().Errorf(ctx, "删除消息失败: %v", err)
		return err
	}
	return nil
}
