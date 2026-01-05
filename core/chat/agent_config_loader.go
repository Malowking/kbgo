package chat

import (
	"context"
	"encoding/json"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/redis/go-redis/v9"
)

// LoadAgentPresetConfig 加载Agent预设配置
func LoadAgentPresetConfig(ctx context.Context, req *v1.ChatReq) *v1.ChatReq {
	// 查询会话信息
	conv, err := dao.Conversation.GetByConvID(ctx, req.ConvID)
	if err != nil {
		g.Log().Warningf(ctx, "查询会话失败，跳过Agent配置加载: %v", err)
		return req
	}
	if conv == nil || conv.AgentPresetID == "" {
		return req
	}

	g.Log().Infof(ctx, "会话关联了Agent预设: %s，开始加载配置", conv.AgentPresetID)

	// 先从缓存获取
	preset, err := cache.GetAgentPreset(ctx, conv.AgentPresetID)
	if err != nil && err != redis.Nil {
		g.Log().Warningf(ctx, "从缓存获取Agent预设失败: %v", err)
	}

	// 缓存未命中，查询数据库
	if preset == nil {
		preset, err = dao.AgentPreset.GetByPresetID(ctx, conv.AgentPresetID)
		if err != nil || preset == nil {
			g.Log().Warningf(ctx, "查询Agent预设失败: %v，使用请求参数", err)
			return req
		}

		// 写入缓存
		cache.SetAgentPreset(ctx, preset)
	}

	// 反序列化配置
	var config v1.AgentConfig
	if err := json.Unmarshal(preset.Config, &config); err != nil {
		g.Log().Warningf(ctx, "Agent配置反序列化失败: %v，使用请求参数", err)
		return req
	}

	g.Log().Infof(ctx, "成功加载Agent预设配置: %s", preset.PresetName)

	// 合并配置：请求参数优先，未指定的参数使用Agent预设
	if req.ModelID == "" {
		req.ModelID = config.ModelID
	}
	if req.EmbeddingModelID == "" {
		req.EmbeddingModelID = config.EmbeddingModelID
	}
	if req.RerankModelID == "" {
		req.RerankModelID = config.RerankModelID
	}
	if req.KnowledgeId == "" {
		req.KnowledgeId = config.KnowledgeId
	}
	if !req.EnableRetriever {
		req.EnableRetriever = config.EnableRetriever
	}
	if req.TopK == 0 {
		req.TopK = config.TopK
	}
	if req.Score == 0 {
		req.Score = config.Score
	}
	if req.RetrieveMode == "" {
		req.RetrieveMode = config.RetrieveMode
	}
	if req.RerankWeight == nil {
		req.RerankWeight = config.RerankWeight
	}
	if !req.JsonFormat {
		req.JsonFormat = config.JsonFormat
	}

	// 新的统一工具配置
	if req.Tools == nil || len(req.Tools) == 0 {
		req.Tools = config.Tools
	}

	// 旧的工具配置字段 (保留以便向后兼容)
	if !req.EnableNL2SQL {
		req.EnableNL2SQL = config.EnableNL2SQL
	}
	if req.NL2SQLDatasource == "" {
		req.NL2SQLDatasource = config.NL2SQLDatasource
	}
	if !req.UseMCP {
		req.UseMCP = config.UseMCP
	}
	if req.MCPServiceTools == nil || len(req.MCPServiceTools) == 0 {
		req.MCPServiceTools = config.MCPServiceTools
	}

	return req
}
