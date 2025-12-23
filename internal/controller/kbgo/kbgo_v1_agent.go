package kbgo

import (
	"context"
	"mime/multipart"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// ============ Agent预设管理接口 ============

// CreateAgentPreset 创建Agent预设
func (c *ControllerV1) CreateAgentPreset(ctx context.Context, req *v1.CreateAgentPresetReq) (res *v1.CreateAgentPresetRes, err error) {
	g.Log().Infof(ctx, "创建Agent预设请求 - UserID: %s, PresetName: %s", req.UserID, req.PresetName)

	agentService := agent.NewAgentService()
	return agentService.CreatePreset(ctx, req)
}

// UpdateAgentPreset 更新Agent预设
func (c *ControllerV1) UpdateAgentPreset(ctx context.Context, req *v1.UpdateAgentPresetReq) (res *v1.UpdateAgentPresetRes, err error) {
	g.Log().Infof(ctx, "更新Agent预设请求 - PresetID: %s, UserID: %s", req.PresetID, req.UserID)

	agentService := agent.NewAgentService()
	return agentService.UpdatePreset(ctx, req)
}

// GetAgentPreset 获取Agent预设详情
func (c *ControllerV1) GetAgentPreset(ctx context.Context, req *v1.GetAgentPresetReq) (res *v1.GetAgentPresetRes, err error) {
	g.Log().Infof(ctx, "获取Agent预设详情 - PresetID: %s", req.PresetID)

	agentService := agent.NewAgentService()
	return agentService.GetPreset(ctx, req.PresetID)
}

// ListAgentPresets 获取Agent预设列表
func (c *ControllerV1) ListAgentPresets(ctx context.Context, req *v1.ListAgentPresetsReq) (res *v1.ListAgentPresetsRes, err error) {
	g.Log().Infof(ctx, "获取Agent预设列表 - UserID: %s, Page: %d, PageSize: %d", req.UserID, req.Page, req.PageSize)

	agentService := agent.NewAgentService()
	return agentService.ListPresets(ctx, req)
}

// DeleteAgentPreset 删除Agent预设
func (c *ControllerV1) DeleteAgentPreset(ctx context.Context, req *v1.DeleteAgentPresetReq) (res *v1.DeleteAgentPresetRes, err error) {
	g.Log().Infof(ctx, "删除Agent预设 - PresetID: %s, UserID: %s", req.PresetID, req.UserID)

	agentService := agent.NewAgentService()
	return agentService.DeletePreset(ctx, req)
}

// ============ Agent调用接口 ============

// AgentChat 使用Agent预设进行对话
func (c *ControllerV1) AgentChat(ctx context.Context, req *v1.AgentChatReq) (res *v1.AgentChatRes, err error) {
	g.Log().Infof(ctx, "Agent对话请求 - PresetID: %s, ConvID: %s, UserID: %s, Question: %s, Stream: %v, Files: %d",
		req.PresetID, req.ConvID, req.UserID, req.Question, req.Stream, len(req.Files))

	// 处理文件上传（如果有文件）
	var uploadedFiles []*common.MultimodalFile
	if len(req.Files) > 0 {
		// 手动从 HTTP Request 中提取文件
		r := g.RequestFromCtx(ctx)
		uploadFiles := r.GetUploadFiles("files")

		// 转换为 multipart.FileHeader 切片
		var fileHeaders []*multipart.FileHeader
		for _, uploadFile := range uploadFiles {
			fileHeaders = append(fileHeaders, uploadFile.FileHeader)
		}

		// 使用 FileUploader 异步上传文件
		fileUploader := common.GetGlobalFileUploader()
		uploadedFiles, err = fileUploader.UploadFiles(ctx, fileHeaders)
		if err != nil {
			g.Log().Errorf(ctx, "文件上传失败: %v", err)
			return nil, err
		}
		g.Log().Infof(ctx, "成功上传 %d 个文件", len(uploadedFiles))
	}

	// 如果启用流式返回，执行流式逻辑
	if req.Stream {
		return nil, c.handleAgentStreamChat(ctx, req, uploadedFiles)
	}

	agentService := agent.NewAgentService()
	return agentService.AgentChat(ctx, req, uploadedFiles)
}

// handleAgentStreamChat 处理Agent流式聊天请求
func (c *ControllerV1) handleAgentStreamChat(ctx context.Context, req *v1.AgentChatReq, uploadedFiles []*common.MultimodalFile) error {
	g.Log().Infof(ctx, "Agent流式对话请求 - PresetID: %s, ConvID: %s, Files: %d", req.PresetID, req.ConvID, len(uploadedFiles))

	agentService := agent.NewAgentService()

	// 获取Agent预设配置
	preset, err := agentService.GetPreset(ctx, req.PresetID)
	if err != nil {
		g.Log().Errorf(ctx, "获取Agent预设失败: %v", err)
		return err
	}

	// 如果没有conv_id，创建新会话
	convID := req.ConvID
	if convID == "" {
		convID = "conv_" + generateUUID()

		// 创建会话记录（与AgentChat保持一致）
		conversation := &gormModel.Conversation{
			ConvID:        convID,
			UserID:        req.UserID,
			Title:         "Agent: " + preset.PresetName,
			ModelName:     preset.Config.ModelID,
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
		ModelID:          preset.Config.ModelID,
		SystemPrompt:     preset.Config.SystemPrompt,
		EmbeddingModelID: preset.Config.EmbeddingModelID,
		RerankModelID:    preset.Config.RerankModelID,
		KnowledgeId:      preset.Config.KnowledgeId,
		EnableRetriever:  preset.Config.EnableRetriever,
		TopK:             preset.Config.TopK,
		Score:            preset.Config.Score,
		RetrieveMode:     preset.Config.RetrieveMode,
		UseMCP:           preset.Config.UseMCP,
		MCPServiceTools:  preset.Config.MCPServiceTools,
		Stream:           true,
		JsonFormat:       preset.Config.JsonFormat,
	}

	// 调用原有的流式Chat处理器，传递上传的文件
	return c.handleStreamChat(ctx, chatReq, uploadedFiles)
}

// generateUUID 生成UUID（简化版）
func generateUUID() string {
	// 这里可以使用uuid库生成
	// 为了简化，暂时返回时间戳
	return "temp_uuid"
}
