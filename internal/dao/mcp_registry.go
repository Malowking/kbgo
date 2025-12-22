package dao

import (
	"context"

	"github.com/Malowking/kbgo/core/errors"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// MCPRegistryDAO MCP注册表数据访问对象
type MCPRegistryDAO struct{}

var MCPRegistry = &MCPRegistryDAO{}

// Create 创建MCP服务注册
func (d *MCPRegistryDAO) Create(ctx context.Context, mcp *gormModel.MCPRegistry) error {
	if err := GetDB().WithContext(ctx).Create(mcp).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to create MCP registry: %v", err)
		return err
	}
	return nil
}

// Update 更新MCP服务注册
func (d *MCPRegistryDAO) Update(ctx context.Context, mcp *gormModel.MCPRegistry) error {
	if err := GetDB().WithContext(ctx).Save(mcp).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to update MCP registry: %v", err)
		return err
	}
	return nil
}

// Delete 删除MCP服务注册
func (d *MCPRegistryDAO) Delete(ctx context.Context, id string) error {
	if err := GetDB().WithContext(ctx).Delete(&gormModel.MCPRegistry{}, "id = ?", id).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to delete MCP registry: %v", err)
		return err
	}
	return nil
}

// GetByID 根据ID查询MCP服务
func (d *MCPRegistryDAO) GetByID(ctx context.Context, id string) (*gormModel.MCPRegistry, error) {
	var mcp gormModel.MCPRegistry
	if err := GetDB().WithContext(ctx).Where("id = ?", id).First(&mcp).Error; err != nil {
		return nil, err
	}
	return &mcp, nil
}

// GetByName 根据名称查询MCP服务
func (d *MCPRegistryDAO) GetByName(ctx context.Context, name string) (*gormModel.MCPRegistry, error) {
	var mcp gormModel.MCPRegistry
	if err := GetDB().WithContext(ctx).Where("name = ?", name).First(&mcp).Error; err != nil {
		return nil, err
	}
	return &mcp, nil
}

// List 查询MCP服务列表
func (d *MCPRegistryDAO) List(ctx context.Context, status *int8, page, pageSize int) ([]*gormModel.MCPRegistry, int64, error) {
	var mcps []*gormModel.MCPRegistry
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.MCPRegistry{})

	// 按状态过滤
	if status != nil {
		query = query.Where("status = ?", *status)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to count MCP registries: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("create_time DESC").Find(&mcps).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list MCP registries: %v", err)
		return nil, 0, err
	}

	return mcps, total, nil
}

// ListActive 查询所有启用的MCP服务
func (d *MCPRegistryDAO) ListActive(ctx context.Context) ([]*gormModel.MCPRegistry, error) {
	var mcps []*gormModel.MCPRegistry
	if err := GetDB().WithContext(ctx).Where("status = ?", 1).Find(&mcps).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list active MCP registries: %v", err)
		return nil, err
	}
	return mcps, nil
}

// UpdateStatus 更新MCP服务状态
func (d *MCPRegistryDAO) UpdateStatus(ctx context.Context, id string, status int8) error {
	if err := GetDB().WithContext(ctx).Model(&gormModel.MCPRegistry{}).
		Where("id = ?", id).
		Update("status", status).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to update MCP registry status: %v", err)
		return err
	}
	return nil
}

// Exists 检查MCP服务名称是否已存在
func (d *MCPRegistryDAO) Exists(ctx context.Context, name string, excludeID ...string) (bool, error) {
	query := GetDB().WithContext(ctx).Model(&gormModel.MCPRegistry{}).Where("name = ?", name)

	// 如果提供了excludeID，则排除该ID（用于更新时检查重名）
	if len(excludeID) > 0 && excludeID[0] != "" {
		query = query.Where("id != ?", excludeID[0])
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, errors.Newf(errors.ErrDatabaseQuery, "failed to check MCP registry existence: %v", err)
	}

	return count > 0, nil
}
