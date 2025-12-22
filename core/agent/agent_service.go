package agent

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/core/chat"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
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
	// 序列化配置为JSON
	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		g.Log().Errorf(ctx, "序列化Agent配置失败: %v", err)
		return nil, errors.Newf(errors.ErrInvalidParameter, "failed to marshal agent config: %v", err)
	}

	// 创建预设对象
	preset := &gormModel.AgentPreset{
		UserID:      req.UserID,
		PresetName:  req.PresetName,
		Description: req.Description,
		Config:      gormModel.JSON(configJSON),
		IsPublic:    req.IsPublic,
	}

	// 保存到数据库
	if err := dao.AgentPreset.Create(ctx, preset); err != nil {
		return nil, errors.Newf(errors.ErrDatabaseInsert, "failed to create agent preset: %v", err)
	}

	g.Log().Infof(ctx, "创建Agent预设成功: %s, User: %s", preset.PresetID, req.UserID)

	// 【新增】创建成功后立即写入Redis缓存
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

	// 构造响应
	res := &v1.GetAgentPresetRes{
		PresetID:    preset.PresetID,
		UserID:      preset.UserID,
		PresetName:  preset.PresetName,
		Description: preset.Description,
		Config:      config,
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

// AgentChat 使用Agent预设进行对话
func (s *AgentService) AgentChat(ctx context.Context, req *v1.AgentChatReq, uploadedFiles []*common.MultimodalFile) (*v1.AgentChatRes, error) {
	// 获取Agent预设配置（带缓存）
	preset, err := s.GetPreset(ctx, req.PresetID)
	if err != nil {
		return nil, err
	}

	// 反序列化配置
	var config v1.AgentConfig
	if err := json.Unmarshal([]byte(fmt.Sprintf("%v", preset.Config)), &config); err != nil {
		// 如果上面的方式失败，直接使用preset.Config
		config = preset.Config
	}

	// 如果没有conv_id，创建新会话
	convID := req.ConvID
	if convID == "" {
		convID = "conv_" + uuid.New().String()

		// 创建会话记录
		conversation := &gormModel.Conversation{
			ConvID:        convID,
			UserID:        req.UserID,
			Title:         "Agent: " + preset.PresetName,
			ModelName:     config.ModelID,
			Status:        "active",
			AgentPresetID: req.PresetID, // 关联Agent预设
		}

		if err := dao.Conversation.Create(ctx, conversation); err != nil {
			g.Log().Warningf(ctx, "创建会话记录失败: %v", err)
			// 不阻断流程，继续执行
		}
	}

	// 构造ChatReq
	chatReq := &v1.ChatReq{
		ConvID:           convID,
		Question:         req.Question,
		ModelID:          config.ModelID,
		SystemPrompt:     config.SystemPrompt,
		EmbeddingModelID: config.EmbeddingModelID,
		RerankModelID:    config.RerankModelID,
		KnowledgeId:      config.KnowledgeId,
		EnableRetriever:  config.EnableRetriever,
		TopK:             config.TopK,
		Score:            config.Score,
		RetrieveMode:     config.RetrieveMode,
		UseMCP:           config.UseMCP,
		MCPServiceTools:  config.MCPServiceTools,
		Stream:           req.Stream,
		JsonFormat:       config.JsonFormat,
	}

	// 调用Chat处理器
	chatHandler := chat.NewChatHandler()
	chatRes, err := chatHandler.Chat(ctx, chatReq, uploadedFiles)
	if err != nil {
		return nil, errors.Newf(errors.ErrChatFailed, "agent chat failed: %v", err)
	}

	// 构造响应
	res := &v1.AgentChatRes{
		ConvID:           convID,
		Answer:           chatRes.Answer,
		ReasoningContent: chatRes.ReasoningContent,
		MCPResults:       chatRes.MCPResults,
	}

	// 转换References
	if len(chatRes.References) > 0 {
		res.References = make([]*v1.AgentDoc, 0, len(chatRes.References))
		for _, ref := range chatRes.References {
			doc := &v1.AgentDoc{
				Content: ref.Content,
				Score:   float64(ref.Score),
			}
			// 从metadata中提取document_id和chunk_id（如果存在）
			if ref.MetaData != nil {
				if docID, ok := ref.MetaData["document_id"].(string); ok {
					doc.DocumentID = docID
				}
				if chunkID, ok := ref.MetaData["chunk_id"].(string); ok {
					doc.ChunkID = chunkID
				}
			}
			res.References = append(res.References, doc)
		}
	}

	return res, nil
}
