package dao

import (
	"context"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// MessageContentDAO 消息内容块数据访问对象
type MessageContentDAO struct{}

var MessageContent = &MessageContentDAO{}

// Create 创建消息内容块
func (d *MessageContentDAO) Create(ctx context.Context, content *gormModel.MessageContent) error {
	if err := GetDB().WithContext(ctx).Create(content).Error; err != nil {
		g.Log().Errorf(ctx, "创建消息内容块失败: %v", err)
		return err
	}
	return nil
}

// CreateBatch 批量创建消息内容块
func (d *MessageContentDAO) CreateBatch(ctx context.Context, contents []*gormModel.MessageContent) error {
	if err := GetDB().WithContext(ctx).CreateInBatches(contents, 100).Error; err != nil {
		g.Log().Errorf(ctx, "批量创建消息内容块失败: %v", err)
		return err
	}
	return nil
}

// ListByMsgID 根据消息ID获取内容块列表
func (d *MessageContentDAO) ListByMsgID(ctx context.Context, msgID string) ([]*gormModel.MessageContent, error) {
	var contents []*gormModel.MessageContent
	if err := GetDB().WithContext(ctx).Where("msg_id = ?", msgID).Order("sort_order ASC").Find(&contents).Error; err != nil {
		g.Log().Errorf(ctx, "查询消息内容块列表失败: %v", err)
		return nil, err
	}
	return contents, nil
}

// ListByMsgIDs 根据多个消息ID获取内容块列表
func (d *MessageContentDAO) ListByMsgIDs(ctx context.Context, msgIDs []string) ([]*gormModel.MessageContent, error) {
	var contents []*gormModel.MessageContent
	if err := GetDB().WithContext(ctx).Where("msg_id IN ?", msgIDs).Order("msg_id, sort_order ASC").Find(&contents).Error; err != nil {
		g.Log().Errorf(ctx, "查询消息内容块列表失败: %v", err)
		return nil, err
	}
	return contents, nil
}

// ListByContentType 根据内容类型获取内容块列表
func (d *MessageContentDAO) ListByContentType(ctx context.Context, contentType string, page, pageSize int) ([]*gormModel.MessageContent, int64, error) {
	var contents []*gormModel.MessageContent
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.MessageContent{}).Where("content_type = ?", contentType)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "统计内容块总数失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&contents).Error; err != nil {
		g.Log().Errorf(ctx, "查询内容块列表失败: %v", err)
		return nil, 0, err
	}

	return contents, total, nil
}

// Update 更新消息内容块
func (d *MessageContentDAO) Update(ctx context.Context, content *gormModel.MessageContent) error {
	if err := GetDB().WithContext(ctx).Save(content).Error; err != nil {
		g.Log().Errorf(ctx, "更新消息内容块失败: %v", err)
		return err
	}
	return nil
}

// Delete 删除消息内容块
func (d *MessageContentDAO) Delete(ctx context.Context, id uint64) error {
	if err := GetDB().WithContext(ctx).Where("id = ?", id).Delete(&gormModel.MessageContent{}).Error; err != nil {
		g.Log().Errorf(ctx, "删除消息内容块失败: %v", err)
		return err
	}
	return nil
}

// DeleteByMsgID 根据消息ID删除内容块
func (d *MessageContentDAO) DeleteByMsgID(ctx context.Context, msgID string) error {
	if err := GetDB().WithContext(ctx).Where("msg_id = ?", msgID).Delete(&gormModel.MessageContent{}).Error; err != nil {
		g.Log().Errorf(ctx, "根据消息ID删除内容块失败: %v", err)
		return err
	}
	return nil
}
