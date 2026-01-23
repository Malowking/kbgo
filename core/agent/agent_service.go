package agent

import (
	"context"
	"encoding/json"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/redis/go-redis/v9"
)

// AgentService Agent业务服务
type AgentService struct{}

// NewAgentService 创建Agent服务实例
func NewAgentService() *AgentService {
	return &AgentService{}
}

// CreatePreset 创建Agent预设
func (s *AgentService) CreatePreset(ctx context.Context, req *v1.CreateAgentPresetReq) (*v1.CreateAgentPresetRes, error) {
	// 调试日志：打印接收到的 Tools 参数
	g.Log().Infof(ctx, "CreatePreset - 接收到的 Tools 参数: %+v", req.Tools)
	if req.Tools != nil {
		g.Log().Infof(ctx, "CreatePreset - Tools 数组长度: %d", len(req.Tools))
		for i, tool := range req.Tools {
			g.Log().Infof(ctx, "CreatePreset - Tool[%d]: Type=%s, Enabled=%v, Config=%+v",
				i, tool.Type, tool.Enabled, tool.Config)
		}
	} else {
		g.Log().Warningf(ctx, "CreatePreset - Tools 参数为 nil")
	}

	// 序列化配置为JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		g.Log().Errorf(ctx, "序列化Agent配置失败: %v", err)
		return nil, errors.Newf(errors.ErrInvalidParameter, "failed to marshal agent config: %v", err)
	}

	// 序列化工具配置为JSON
	var toolsJSON gormModel.JSON
	if req.Tools != nil && len(req.Tools) > 0 {
		toolsBytes, err := json.Marshal(req.Tools)
		if err != nil {
			g.Log().Errorf(ctx, "序列化Tools配置失败: %v", err)
			return nil, errors.Newf(errors.ErrInvalidParameter, "failed to marshal tools config: %v", err)
		}
		toolsJSON = gormModel.JSON(toolsBytes)
		g.Log().Infof(ctx, "CreatePreset - 序列化后的 Tools JSON: %s", string(toolsJSON))
	} else {
		g.Log().Warningf(ctx, "CreatePreset - Tools 为空或长度为0，不进行序列化")
	}

	// 创建预设对象
	preset := &gormModel.AgentPreset{
		UserID:      req.UserID,
		PresetName:  req.PresetName,
		Description: req.Description,
		Config:      gormModel.JSON(configJSON),
		Tools:       toolsJSON,
		IsPublic:    req.IsPublic,
	}

	// 保存到数据库
	if err := dao.AgentPreset.Create(ctx, preset); err != nil {
		return nil, errors.Newf(errors.ErrDatabaseInsert, "failed to create agent preset: %v", err)
	}

	g.Log().Infof(ctx, "创建Agent预设成功: %s, User: %s", preset.PresetID, req.UserID)

	// 创建成功后立即写入Redis缓存
	if err := cache.SetAgentPreset(ctx, preset); err != nil {
		g.Log().Warningf(ctx, "写入Agent预设缓存失败（非致命）: %v", err)
		// 不阻断流程，缓存失败不影响返回
	}

	return &v1.CreateAgentPresetRes{
		PresetID: preset.PresetID,
	}, nil
}

// UpdatePreset 更新Agent预设
func (s *AgentService) UpdatePreset(ctx context.Context, req *v1.UpdateAgentPresetReq) (*v1.UpdateAgentPresetRes, error) {
	// 检查权限
	isOwner, err := dao.AgentPreset.CheckOwnership(ctx, req.PresetID, req.UserID)
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to check ownership: %v", err)
	}
	if !isOwner {
		return nil, errors.New(errors.ErrUnauthorized, "no permission to modify this preset")
	}

	// 查询现有预设
	preset, err := dao.AgentPreset.GetByPresetID(ctx, req.PresetID)
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to query preset: %v", err)
	}
	if preset == nil {
		return nil, errors.Newf(errors.ErrNotFound, "preset not found: %s", req.PresetID)
	}

	// 更新字段
	updates := make(map[string]interface{})
	if req.PresetName != "" {
		updates["preset_name"] = req.PresetName
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	updates["is_public"] = req.IsPublic

	// 如果有配置更新
	if req.Config.ModelID != "" {
		configJSON, err := json.Marshal(req.Config)
		if err != nil {
			return nil, errors.Newf(errors.ErrInvalidParameter, "failed to marshal config: %v", err)
		}
		updates["config"] = gormModel.JSON(configJSON)
	}

	// 如果有工具配置更新
	if req.Tools != nil {
		if len(req.Tools) > 0 {
			toolsJSON, err := json.Marshal(req.Tools)
			if err != nil {
				return nil, errors.Newf(errors.ErrInvalidParameter, "failed to marshal tools: %v", err)
			}
			updates["tools"] = gormModel.JSON(toolsJSON)
		} else {
			// 空数组表示清空工具配置
			updates["tools"] = gormModel.JSON(nil)
		}
	}

	// 执行更新
	if err := dao.AgentPreset.UpdateFields(ctx, req.PresetID, updates); err != nil {
		return nil, errors.Newf(errors.ErrDatabaseUpdate, "failed to update preset: %v", err)
	}

	// 清除缓存
	cache.InvalidateAgentPreset(ctx, req.PresetID)

	g.Log().Infof(ctx, "更新Agent预设成功: %s", req.PresetID)

	return &v1.UpdateAgentPresetRes{
		Success: true,
	}, nil
}

// GetPreset 获取Agent预设详情
func (s *AgentService) GetPreset(ctx context.Context, presetID string) (*v1.GetAgentPresetRes, error) {
	// 先从缓存获取
	preset, err := cache.GetAgentPreset(ctx, presetID)
	if err != nil && err != redis.Nil {
		g.Log().Warningf(ctx, "从缓存获取Agent预设失败: %v", err)
	}

	// 缓存未命中，查询数据库
	if preset == nil {
		preset, err = dao.AgentPreset.GetByPresetID(ctx, presetID)
		if err != nil {
			return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to query preset: %v", err)
		}
		if preset == nil {
			return nil, errors.Newf(errors.ErrNotFound, "preset not found: %s", presetID)
		}

		// 写入缓存
		if err := cache.SetAgentPreset(ctx, preset); err != nil {
			g.Log().Warningf(ctx, "写入Agent预设缓存失败: %v", err)
		}
	}

	// 反序列化配置
	var config v1.AgentConfig
	if err := json.Unmarshal(preset.Config, &config); err != nil {
		return nil, errors.Newf(errors.ErrInvalidParameter, "failed to unmarshal config: %v", err)
	}

	// 反序列化工具配置
	var tools []*v1.ToolConfig
	if preset.Tools != nil && len(preset.Tools) > 0 {
		if err := json.Unmarshal(preset.Tools, &tools); err != nil {
			g.Log().Warningf(ctx, "反序列化Tools配置失败: %v", err)
			// 不阻断流程，工具配置解析失败时返回空数组
			tools = nil
		}
	}

	// 构造响应
	res := &v1.GetAgentPresetRes{
		PresetID:    preset.PresetID,
		UserID:      preset.UserID,
		PresetName:  preset.PresetName,
		Description: preset.Description,
		Config:      config,
		Tools:       tools,
		IsPublic:    preset.IsPublic,
	}

	if preset.CreateTime != nil {
		res.CreateTime = preset.CreateTime.Format("2006-01-02 15:04:05")
	}
	if preset.UpdateTime != nil {
		res.UpdateTime = preset.UpdateTime.Format("2006-01-02 15:04:05")
	}

	return res, nil
}

// ListPresets 获取Agent预设列表
func (s *AgentService) ListPresets(ctx context.Context, req *v1.ListAgentPresetsReq) (*v1.ListAgentPresetsRes, error) {
	var presets []*gormModel.AgentPreset
	var total int64
	var err error

	// 默认分页参数
	page := req.Page
	if page < 1 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize < 1 {
		pageSize = 10
	}

	// 根据条件查询
	if req.IsPublic != nil && *req.IsPublic {
		// 查询公开预设
		presets, total, err = dao.AgentPreset.ListPublic(ctx, page, pageSize)
	} else if req.UserID != "" {
		// 查询用户的预设
		presets, total, err = dao.AgentPreset.ListByUserID(ctx, req.UserID, page, pageSize)
	} else {
		return nil, errors.New(errors.ErrInvalidParameter, "must specify user_id or is_public parameter")
	}

	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to query preset list: %v", err)
	}

	// 构造响应列表
	list := make([]*v1.AgentPresetItem, 0, len(presets))
	for _, preset := range presets {
		item := &v1.AgentPresetItem{
			PresetID:    preset.PresetID,
			PresetName:  preset.PresetName,
			Description: preset.Description,
			IsPublic:    preset.IsPublic,
		}
		if preset.CreateTime != nil {
			item.CreateTime = preset.CreateTime.Format("2006-01-02 15:04:05")
		}
		if preset.UpdateTime != nil {
			item.UpdateTime = preset.UpdateTime.Format("2006-01-02 15:04:05")
		}
		list = append(list, item)
	}

	return &v1.ListAgentPresetsRes{
		List:  list,
		Total: total,
		Page:  page,
	}, nil
}

// DeletePreset 删除Agent预设
func (s *AgentService) DeletePreset(ctx context.Context, req *v1.DeleteAgentPresetReq) (*v1.DeleteAgentPresetRes, error) {
	// 检查权限
	isOwner, err := dao.AgentPreset.CheckOwnership(ctx, req.PresetID, req.UserID)
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseQuery, "failed to check ownership: %v", err)
	}
	if !isOwner {
		return nil, errors.New(errors.ErrUnauthorized, "no permission to delete this preset")
	}

	// 删除预设
	if err := dao.AgentPreset.Delete(ctx, req.PresetID); err != nil {
		return nil, errors.Newf(errors.ErrDatabaseDelete, "failed to delete preset: %v", err)
	}

	// 清除缓存
	cache.InvalidateAgentPreset(ctx, req.PresetID)

	g.Log().Infof(ctx, "删除Agent预设成功: %s", req.PresetID)

	return &v1.DeleteAgentPresetRes{
		Success: true,
	}, nil
}
