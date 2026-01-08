package dao

import (
	"context"

	"github.com/Malowking/kbgo/core/errors"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// ClaudeSkillDAO Skill 数据访问对象
type ClaudeSkillDAO struct{}

var ClaudeSkill = &ClaudeSkillDAO{}

// Create 创建 Skill
func (d *ClaudeSkillDAO) Create(ctx context.Context, skill *gormModel.ClaudeSkill) error {
	if err := GetDB().WithContext(ctx).Create(skill).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to create skill: %v", err)
		return errors.Newf(errors.ErrDatabaseQuery, "创建 Skill 失败: %v", err)
	}
	return nil
}

// Update 更新 Skill
func (d *ClaudeSkillDAO) Update(ctx context.Context, skill *gormModel.ClaudeSkill) error {
	if err := GetDB().WithContext(ctx).Save(skill).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to update skill: %v", err)
		return errors.Newf(errors.ErrDatabaseQuery, "更新 Skill 失败: %v", err)
	}
	return nil
}

// Delete 删除 Skill
func (d *ClaudeSkillDAO) Delete(ctx context.Context, id string) error {
	if err := GetDB().WithContext(ctx).Delete(&gormModel.ClaudeSkill{}, "id = ?", id).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to delete skill: %v", err)
		return errors.Newf(errors.ErrDatabaseQuery, "删除 Skill 失败: %v", err)
	}
	return nil
}

// GetByID 根据ID查询 Skill
func (d *ClaudeSkillDAO) GetByID(ctx context.Context, id string) (*gormModel.ClaudeSkill, error) {
	var skill gormModel.ClaudeSkill
	if err := GetDB().WithContext(ctx).Where("id = ?", id).First(&skill).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

// GetByName 根据名称查询 Skill
func (d *ClaudeSkillDAO) GetByName(ctx context.Context, name string, ownerID string) (*gormModel.ClaudeSkill, error) {
	var skill gormModel.ClaudeSkill
	query := GetDB().WithContext(ctx).Where("name = ?", name)

	// 如果指定了 ownerID，只查询该用户的 Skill
	if ownerID != "" {
		query = query.Where("owner_id = ?", ownerID)
	}

	if err := query.First(&skill).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

// List 查询 Skill 列表
func (d *ClaudeSkillDAO) List(ctx context.Context, req *ListSkillsReq) ([]*gormModel.ClaudeSkill, int64, error) {
	var skills []*gormModel.ClaudeSkill
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.ClaudeSkill{})

	// 按状态过滤
	if req.Status != nil {
		query = query.Where("status = ?", *req.Status)
	}

	// 按分类过滤
	if req.Category != "" {
		query = query.Where("category = ?", req.Category)
	}

	// 按所有者过滤
	if req.OwnerID != "" {
		if req.IncludePublic {
			// 查询自己的 + 公开的
			query = query.Where("owner_id = ? OR is_public = ?", req.OwnerID, true)
		} else {
			// 只查询自己的
			query = query.Where("owner_id = ?", req.OwnerID)
		}
	} else if req.PublicOnly {
		// 只查询公开的
		query = query.Where("is_public = ?", true)
	}

	// 关键词搜索
	if req.Keyword != "" {
		keyword := "%" + req.Keyword + "%"
		query = query.Where("name LIKE ? OR description LIKE ? OR tags LIKE ?", keyword, keyword, keyword)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to count skills: %v", err)
		return nil, 0, errors.Newf(errors.ErrDatabaseQuery, "统计 Skill 失败: %v", err)
	}

	// 排序
	orderBy := "create_time DESC"
	if req.OrderBy != "" {
		orderBy = req.OrderBy
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).Order(orderBy).Find(&skills).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list skills: %v", err)
		return nil, 0, errors.Newf(errors.ErrDatabaseQuery, "查询 Skill 列表失败: %v", err)
	}

	return skills, total, nil
}

// ListSkillsReq 查询 Skill 列表请求
type ListSkillsReq struct {
	Status        *int8  // 状态过滤
	Category      string // 分类过滤
	OwnerID       string // 所有者ID
	IncludePublic bool   // 是否包含公开的 Skill
	PublicOnly    bool   // 只查询公开的 Skill
	Keyword       string // 关键词搜索
	OrderBy       string // 排序字段
	Page          int    // 页码
	PageSize      int    // 每页数量
}

// ListActive 查询所有启用的 Skill
func (d *ClaudeSkillDAO) ListActive(ctx context.Context, ownerID string) ([]*gormModel.ClaudeSkill, error) {
	var skills []*gormModel.ClaudeSkill
	query := GetDB().WithContext(ctx).Where("status = ?", 1)

	// 查询自己的 + 公开的
	if ownerID != "" {
		query = query.Where("owner_id = ? OR is_public = ?", ownerID, true)
	}

	if err := query.Find(&skills).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list active skills: %v", err)
		return nil, errors.Newf(errors.ErrDatabaseQuery, "查询启用的 Skill 失败: %v", err)
	}
	return skills, nil
}

// UpdateStatus 更新 Skill 状态
func (d *ClaudeSkillDAO) UpdateStatus(ctx context.Context, id string, status int8) error {
	if err := GetDB().WithContext(ctx).Model(&gormModel.ClaudeSkill{}).
		Where("id = ?", id).
		Update("status", status).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to update skill status: %v", err)
		return errors.Newf(errors.ErrDatabaseQuery, "更新 Skill 状态失败: %v", err)
	}
	return nil
}

// UpdateStats 更新 Skill 统计信息
func (d *ClaudeSkillDAO) UpdateStats(ctx context.Context, id string, success bool, duration int64) error {
	// 使用原生 SQL 更新计数器
	sql := `
		UPDATE claude_skills
		SET
			call_count = call_count + 1,
			success_count = CASE WHEN ? THEN success_count + 1 ELSE success_count END,
			fail_count = CASE WHEN ? THEN fail_count ELSE fail_count + 1 END,
			avg_duration = (avg_duration * call_count + ?) / (call_count + 1),
			last_used_at = NOW()
		WHERE id = ?
	`

	if err := GetDB().WithContext(ctx).Exec(sql, success, success, duration, id).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to update skill stats: %v", err)
		return errors.Newf(errors.ErrDatabaseQuery, "更新 Skill 统计失败: %v", err)
	}
	return nil
}

// Exists 检查 Skill 名称是否已存在
func (d *ClaudeSkillDAO) Exists(ctx context.Context, name string, ownerID string, excludeID ...string) (bool, error) {
	query := GetDB().WithContext(ctx).Model(&gormModel.ClaudeSkill{}).
		Where("name = ? AND owner_id = ?", name, ownerID)

	// 如果提供了 excludeID，则排除该ID
	if len(excludeID) > 0 && excludeID[0] != "" {
		query = query.Where("id != ?", excludeID[0])
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		return false, errors.Newf(errors.ErrDatabaseQuery, "检查 Skill 是否存在失败: %v", err)
	}

	return count > 0, nil
}

// GetByScriptHash 根据脚本哈希查询 Skill（用于去重）
func (d *ClaudeSkillDAO) GetByScriptHash(ctx context.Context, scriptHash string, ownerID string) (*gormModel.ClaudeSkill, error) {
	var skill gormModel.ClaudeSkill
	if err := GetDB().WithContext(ctx).
		Where("script_hash = ? AND owner_id = ?", scriptHash, ownerID).
		First(&skill).Error; err != nil {
		return nil, err
	}
	return &skill, nil
}

// ClaudeSkillCallLogDAO Skill 调用日志数据访问对象
type ClaudeSkillCallLogDAO struct{}

var ClaudeSkillCallLog = &ClaudeSkillCallLogDAO{}

// Create 创建调用日志
func (d *ClaudeSkillCallLogDAO) Create(ctx context.Context, log *gormModel.ClaudeSkillCallLog) error {
	if err := GetDB().WithContext(ctx).Create(log).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to create skill call log: %v", err)
		return errors.Newf(errors.ErrDatabaseQuery, "创建调用日志失败: %v", err)
	}
	return nil
}

// List 查询调用日志列表
func (d *ClaudeSkillCallLogDAO) List(ctx context.Context, req *ListSkillCallLogsReq) ([]*gormModel.ClaudeSkillCallLog, int64, error) {
	var logs []*gormModel.ClaudeSkillCallLog
	var total int64

	query := GetDB().WithContext(ctx).Model(&gormModel.ClaudeSkillCallLog{})

	// 按 Skill ID 过滤
	if req.SkillID != "" {
		query = query.Where("skill_id = ?", req.SkillID)
	}

	// 按会话ID过滤
	if req.ConversationID != "" {
		query = query.Where("conversation_id = ?", req.ConversationID)
	}

	// 按成功状态过滤
	if req.Success != nil {
		query = query.Where("success = ?", *req.Success)
	}

	// 统计总数
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to count skill call logs: %v", err)
		return nil, 0, errors.Newf(errors.ErrDatabaseQuery, "统计调用日志失败: %v", err)
	}

	// 分页查询
	offset := (req.Page - 1) * req.PageSize
	if err := query.Offset(offset).Limit(req.PageSize).Order("create_time DESC").Find(&logs).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list skill call logs: %v", err)
		return nil, 0, errors.Newf(errors.ErrDatabaseQuery, "查询调用日志失败: %v", err)
	}

	return logs, total, nil
}

// ListSkillCallLogsReq 查询调用日志请求
type ListSkillCallLogsReq struct {
	SkillID        string // Skill ID
	ConversationID string // 会话ID
	Success        *bool  // 成功状态
	Page           int    // 页码
	PageSize       int    // 每页数量
}
