package dao

import (
	"context"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// AgentPresetDAO Agent预设数据访问对象
type AgentPresetDAO struct{}

var AgentPreset = &AgentPresetDAO{}

// Create 创建Agent预设
func (d *AgentPresetDAO) Create(ctx context.Context, preset *gormModel.AgentPreset) error {
	if err := GetDB().WithContext(ctx).Create(preset).Error; err != nil {
		g.Log().Errorf(ctx, "创建Agent预设失败: %v", err)
		return err
	}
	return nil
}

// GetByPresetID 根据预设ID获取Agent预设
func (d *AgentPresetDAO) GetByPresetID(ctx context.Context, presetID string) (*gormModel.AgentPreset, error) {
	var preset gormModel.AgentPreset
	if err := GetDB().WithContext(ctx).Where("preset_id = ?", presetID).First(&preset).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		g.Log().Errorf(ctx, "查询Agent预设失败: %v", err)
		return nil, err
	}
	return &preset, nil
}

// ListByUserID 根据用户ID获取Agent预设列表
func (d *AgentPresetDAO) ListByUserID(ctx context.Context, userID string, page, pageSize int) ([]*gormModel.AgentPreset, int64, error) {
	var presets []*gormModel.AgentPreset
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.AgentPreset{}).Where("user_id = ?", userID)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "统计Agent预设总数失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("update_time DESC").Find(&presets).Error; err != nil {
		g.Log().Errorf(ctx, "查询Agent预设列表失败: %v", err)
		return nil, 0, err
	}

	return presets, total, nil
}

// ListPublic 获取公开的Agent预设列表
func (d *AgentPresetDAO) ListPublic(ctx context.Context, page, pageSize int) ([]*gormModel.AgentPreset, int64, error) {
	var presets []*gormModel.AgentPreset
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.AgentPreset{}).Where("is_public = ?", true)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "统计公开Agent预设总数失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("update_time DESC").Find(&presets).Error; err != nil {
		g.Log().Errorf(ctx, "查询公开Agent预设列表失败: %v", err)
		return nil, 0, err
	}

	return presets, total, nil
}

// Update 更新Agent预设
func (d *AgentPresetDAO) Update(ctx context.Context, preset *gormModel.AgentPreset) error {
	if err := GetDB().WithContext(ctx).Save(preset).Error; err != nil {
		g.Log().Errorf(ctx, "更新Agent预设失败: %v", err)
		return err
	}
	return nil
}

// UpdateFields 更新Agent预设指定字段
func (d *AgentPresetDAO) UpdateFields(ctx context.Context, presetID string, updates map[string]interface{}) error {
	if err := GetDB().WithContext(ctx).Model(&gormModel.AgentPreset{}).Where("preset_id = ?", presetID).Updates(updates).Error; err != nil {
		g.Log().Errorf(ctx, "更新Agent预设字段失败: %v", err)
		return err
	}
	return nil
}

// Delete 删除Agent预设
func (d *AgentPresetDAO) Delete(ctx context.Context, presetID string) error {
	if err := GetDB().WithContext(ctx).Where("preset_id = ?", presetID).Delete(&gormModel.AgentPreset{}).Error; err != nil {
		g.Log().Errorf(ctx, "删除Agent预设失败: %v", err)
		return err
	}
	return nil
}

// CheckOwnership 检查Agent预设是否属于指定用户
func (d *AgentPresetDAO) CheckOwnership(ctx context.Context, presetID, userID string) (bool, error) {
	var count int64
	if err := GetDB().WithContext(ctx).Model(&gormModel.AgentPreset{}).
		Where("preset_id = ? AND user_id = ?", presetID, userID).
		Count(&count).Error; err != nil {
		g.Log().Errorf(ctx, "检查Agent预设所有权失败: %v", err)
		return false, err
	}
	return count > 0, nil
}
