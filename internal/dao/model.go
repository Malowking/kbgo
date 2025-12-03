package dao

import (
	"context"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// AIModelDAO AI模型数据访问对象
type AIModelDAO struct{}

var AIModel = &AIModelDAO{}

// Create 创建AI模型
func (d *AIModelDAO) Create(ctx context.Context, model *gormModel.AIModel) error {
	if err := GetDB().WithContext(ctx).Create(model).Error; err != nil {
		g.Log().Errorf(ctx, "创建AI模型失败: %v", err)
		return err
	}
	return nil
}

// GetByID 根据模型ID获取模型
func (d *AIModelDAO) GetByID(ctx context.Context, modelID string) (*gormModel.AIModel, error) {
	var model gormModel.AIModel
	if err := GetDB().WithContext(ctx).Where("model_id = ?", modelID).First(&model).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		g.Log().Errorf(ctx, "查询AI模型失败: %v", err)
		return nil, err
	}
	return &model, nil
}

// List 获取模型列表
func (d *AIModelDAO) List(ctx context.Context, modelType string, enabled *bool, page, pageSize int) ([]*gormModel.AIModel, int64, error) {
	var models []*gormModel.AIModel
	var total int64

	db := GetDB().WithContext(ctx).Model(&gormModel.AIModel{})

	// 添加过滤条件
	if modelType != "" {
		db = db.Where("model_type = ?", modelType)
	}
	if enabled != nil {
		db = db.Where("enabled = ?", *enabled)
	}

	// 获取总数
	if err := db.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "查询AI模型总数失败: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := db.Offset(offset).Limit(pageSize).Order("create_time DESC").Find(&models).Error; err != nil {
		g.Log().Errorf(ctx, "查询AI模型列表失败: %v", err)
		return nil, 0, err
	}

	return models, total, nil
}

// ListEnabled 获取所有启用的模型
func (d *AIModelDAO) ListEnabled(ctx context.Context) ([]*gormModel.AIModel, error) {
	var models []*gormModel.AIModel
	if err := GetDB().WithContext(ctx).Where("enabled = ?", true).Order("create_time DESC").Find(&models).Error; err != nil {
		g.Log().Errorf(ctx, "查询启用的AI模型失败: %v", err)
		return nil, err
	}
	return models, nil
}

// Update 更新AI模型
func (d *AIModelDAO) Update(ctx context.Context, model *gormModel.AIModel) error {
	if err := GetDB().WithContext(ctx).Save(model).Error; err != nil {
		g.Log().Errorf(ctx, "更新AI模型失败: %v", err)
		return err
	}
	return nil
}

// Delete 删除AI模型（硬删除）
func (d *AIModelDAO) Delete(ctx context.Context, modelID string) error {
	if err := GetDB().WithContext(ctx).Where("model_id = ?", modelID).Delete(&gormModel.AIModel{}).Error; err != nil {
		g.Log().Errorf(ctx, "删除AI模型失败: %v", err)
		return err
	}
	return nil
}

// GetByType 根据类型获取模型列表
func (d *AIModelDAO) GetByType(ctx context.Context, modelType string) ([]*gormModel.AIModel, error) {
	var models []*gormModel.AIModel
	if err := GetDB().WithContext(ctx).Where("model_type = ? AND enabled = ?", modelType, true).Order("create_time DESC").Find(&models).Error; err != nil {
		g.Log().Errorf(ctx, "根据类型查询AI模型失败: %v", err)
		return nil, err
	}
	return models, nil
}

// GetAll 获取所有模型（用于Registry加载）
func (d *AIModelDAO) GetAll(ctx context.Context) ([]*gormModel.AIModel, error) {
	var models []*gormModel.AIModel
	if err := GetDB().WithContext(ctx).Order("create_time DESC").Find(&models).Error; err != nil {
		g.Log().Errorf(ctx, "查询所有AI模型失败: %v", err)
		return nil, err
	}
	return models, nil
}
