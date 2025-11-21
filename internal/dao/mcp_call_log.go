package dao

import (
	"context"
	"time"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// MCPCallLogDAO MCP调用日志数据访问对象
type MCPCallLogDAO struct{}

var MCPCallLog = &MCPCallLogDAO{}

// Create 创建MCP调用日志
func (d *MCPCallLogDAO) Create(ctx context.Context, log *gormModel.MCPCallLog) error {
	if err := GetDB().WithContext(ctx).Create(log).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to create MCP call log: %v", err)
		return err
	}
	return nil
}

// GetByID 根据ID查询调用日志
func (d *MCPCallLogDAO) GetByID(ctx context.Context, id string) (*gormModel.MCPCallLog, error) {
	var log gormModel.MCPCallLog
	if err := GetDB().WithContext(ctx).Where("id = ?", id).First(&log).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// ListByConversationID 根据对话ID查询调用日志
func (d *MCPCallLogDAO) ListByConversationID(ctx context.Context, conversationID string, page, pageSize int) ([]*gormModel.MCPCallLog, int64, error) {
	var logs []*gormModel.MCPCallLog
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.MCPCallLog{}).Where("conversation_id = ?", conversationID)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to count MCP call logs: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("create_time DESC").Find(&logs).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list MCP call logs: %v", err)
		return nil, 0, err
	}

	return logs, total, nil
}

// ListByMCPRegistry 根据MCP服务ID查询调用日志
func (d *MCPCallLogDAO) ListByMCPRegistry(ctx context.Context, registryID string, page, pageSize int) ([]*gormModel.MCPCallLog, int64, error) {
	var logs []*gormModel.MCPCallLog
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.MCPCallLog{}).Where("mcp_registry_id = ?", registryID)

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to count MCP call logs: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("create_time DESC").Find(&logs).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list MCP call logs: %v", err)
		return nil, 0, err
	}

	return logs, total, nil
}

// List 查询MCP调用日志列表（支持多条件过滤）
func (d *MCPCallLogDAO) List(ctx context.Context, filter *MCPCallLogFilter, page, pageSize int) ([]*gormModel.MCPCallLog, int64, error) {
	var logs []*gormModel.MCPCallLog
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.MCPCallLog{})

	// 应用过滤条件
	if filter != nil {
		if filter.ConversationID != "" {
			query = query.Where("conversation_id = ?", filter.ConversationID)
		}
		if filter.MCPRegistryID != "" {
			query = query.Where("mcp_registry_id = ?", filter.MCPRegistryID)
		}
		if filter.MCPServiceName != "" {
			query = query.Where("mcp_service_name = ?", filter.MCPServiceName)
		}
		if filter.ToolName != "" {
			query = query.Where("tool_name = ?", filter.ToolName)
		}
		if filter.Status != nil {
			query = query.Where("status = ?", *filter.Status)
		}
		if filter.StartTime != nil {
			query = query.Where("create_time >= ?", filter.StartTime)
		}
		if filter.EndTime != nil {
			query = query.Where("create_time <= ?", filter.EndTime)
		}
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to count MCP call logs: %v", err)
		return nil, 0, err
	}

	// 分页查询
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("create_time DESC").Find(&logs).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list MCP call logs: %v", err)
		return nil, 0, err
	}

	return logs, total, nil
}

// DeleteByConversationID 根据对话ID删除调用日志
func (d *MCPCallLogDAO) DeleteByConversationID(ctx context.Context, conversationID string) error {
	if err := GetDB().WithContext(ctx).Where("conversation_id = ?", conversationID).Delete(&gormModel.MCPCallLog{}).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to delete MCP call logs by conversation ID: %v", err)
		return err
	}
	return nil
}

// GetStatsByMCPRegistry 获取MCP服务的调用统计
func (d *MCPCallLogDAO) GetStatsByMCPRegistry(ctx context.Context, registryID string) (*MCPCallStats, error) {
	var stats MCPCallStats

	// 查询总调用次数
	if err := GetDB().WithContext(ctx).Model(&gormModel.MCPCallLog{}).
		Where("mcp_registry_id = ?", registryID).
		Count(&stats.TotalCalls).Error; err != nil {
		return nil, err
	}

	// 查询成功次数
	if err := GetDB().WithContext(ctx).Model(&gormModel.MCPCallLog{}).
		Where("mcp_registry_id = ? AND status = ?", registryID, 1).
		Count(&stats.SuccessCalls).Error; err != nil {
		return nil, err
	}

	// 查询失败次数
	if err := GetDB().WithContext(ctx).Model(&gormModel.MCPCallLog{}).
		Where("mcp_registry_id = ? AND status = ?", registryID, 0).
		Count(&stats.FailedCalls).Error; err != nil {
		return nil, err
	}

	// 查询平均耗时
	if err := GetDB().WithContext(ctx).Model(&gormModel.MCPCallLog{}).
		Where("mcp_registry_id = ? AND status = ?", registryID, 1).
		Select("AVG(duration)").
		Scan(&stats.AvgDuration).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}

// MCPCallLogFilter 调用日志过滤条件
type MCPCallLogFilter struct {
	ConversationID string
	MCPRegistryID  string
	MCPServiceName string
	ToolName       string
	Status         *int8
	StartTime      *time.Time
	EndTime        *time.Time
}

// MCPCallStats MCP调用统计
type MCPCallStats struct {
	TotalCalls   int64   // 总调用次数
	SuccessCalls int64   // 成功次数
	FailedCalls  int64   // 失败次数
	AvgDuration  float64 // 平均耗时（毫秒）
}
