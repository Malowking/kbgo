package kbgo

import (
	"context"
	"mime/multipart"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent"
	"github.com/Malowking/kbgo/core/chat"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/gogf/gf/v2/errors/gerror"
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

	// 获取Agent预设配置
	agentService := agent.NewAgentService()
	preset, err := agentService.GetPreset(ctx, req.PresetID)
	if err != nil {
		return nil, err
	}

	// 验证conv_id：前端必须传递conv_id
	convID := req.ConvID
	if convID == "" {
		g.Log().Errorf(ctx, "conv_id不能为空")
		return nil, gerror.New("会话ID不能为空")
	}

	// 检查对话是否存在
	existingConv, err := dao.Conversation.GetByConvID(ctx, convID)
	if err != nil {
		g.Log().Errorf(ctx, "查询会话失败: %v", err)
		return nil, gerror.Newf("查询会话失败: %v", err)
	}

	// 如果对话不存在，返回错误
	if existingConv == nil {
		g.Log().Errorf(ctx, "会话不存在: %s", convID)
		return nil, gerror.Newf("会话不存在")
	}

	g.Log().Infof(ctx, "使用会话: %s, type: %s", convID, existingConv.ConversationType)

	// 构造ChatReq
	chatReq := &v1.ChatReq{
		ConvID:          convID,
		Question:        req.Question,
		ModelID:         preset.Config.ModelID,
		SystemPrompt:    preset.Config.SystemPrompt,
		RerankModelID:   preset.Config.RerankModelID,
		KnowledgeId:     preset.Config.KnowledgeId,
		EnableRetriever: preset.Config.EnableRetriever,
		TopK:            preset.Config.TopK,
		Score:           preset.Config.Score,
		RetrieveMode:    preset.Config.RetrieveMode,
		Stream:          req.Stream,
		JsonFormat:      preset.Config.JsonFormat,
		Tools:           preset.Tools,
	}

	// 调用Chat处理器
	chatHandler := chat.NewChatHandler()
	chatRes, err := chatHandler.Chat(ctx, chatReq, uploadedFiles)
	if err != nil {
		return nil, err
	}

	// 构造响应
	res = &v1.AgentChatRes{
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
			// 从metadata中提取document_id和chunk_id
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

// handleAgentStreamChat 处理Agent流式聊天请求
func (c *ControllerV1) handleAgentStreamChat(ctx context.Context, req *v1.AgentChatReq, uploadedFiles []*common.MultimodalFile) error {
	agentService := agent.NewAgentService()

	// 获取Agent预设配置
	preset, err := agentService.GetPreset(ctx, req.PresetID)
	if err != nil {
		g.Log().Errorf(ctx, "获取Agent预设失败: %v", err)
		return err
	}

	// 验证conv_id：前端必须传递conv_id
	convID := req.ConvID
	if convID == "" {
		g.Log().Errorf(ctx, "conv_id不能为空")
		return gerror.New("会话ID不能为空")
	}

	// 检查对话是否存在
	existingConv, err := dao.Conversation.GetByConvID(ctx, convID)
	if err != nil {
		g.Log().Errorf(ctx, "查询会话失败: %v", err)
		return gerror.Newf("查询会话失败: %v", err)
	}

	// 如果对话不存在，返回错误
	if existingConv == nil {
		g.Log().Errorf(ctx, "会话不存在: %s", convID)
		return gerror.Newf("会话不存在")
	}

	g.Log().Infof(ctx, "使用会话: %s, type: %s", convID, existingConv.ConversationType)

	// 构造ChatReq
	chatReq := &v1.ChatReq{
		ConvID:          convID,
		Question:        req.Question,
		ModelID:         preset.Config.ModelID,
		SystemPrompt:    preset.Config.SystemPrompt,
		RerankModelID:   preset.Config.RerankModelID,
		KnowledgeId:     preset.Config.KnowledgeId,
		EnableRetriever: preset.Config.EnableRetriever,
		TopK:            preset.Config.TopK,
		Score:           preset.Config.Score,
		RetrieveMode:    preset.Config.RetrieveMode,
		Stream:          true,
		JsonFormat:      preset.Config.JsonFormat,
		Tools:           preset.Tools,
	}

	// 调用原有的流式Chat处理器，传递上传的文件
	return c.handleStreamChat(ctx, chatReq, uploadedFiles)
}
