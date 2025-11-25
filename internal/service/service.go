package service

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/core/config"
	"github.com/gogf/gf/v2/frame/g"
)

var sharedConfig *config.Config

// InitSharedServices 初始化共享服务配置
func InitSharedServices(ctx context.Context) error {
	// 获取向量数据库类型配置
	vectorDBType := g.Cfg().MustGet(ctx, "vectordb.type", "milvus").String()

	// 创建共享配置（不包含具体的数据库客户端）
	sharedConfig = &config.Config{
		// 向量数据库配置
		Database: g.Cfg().MustGet(ctx, fmt.Sprintf("%s.database", vectorDBType)).String(),

		// Embedding 配置
		APIKey:         g.Cfg().MustGet(ctx, "embedding.apiKey").String(),
		BaseURL:        g.Cfg().MustGet(ctx, "embedding.baseURL").String(),
		EmbeddingModel: g.Cfg().MustGet(ctx, "embedding.model").String(),

		// Chat 配置
		ChatModel: g.Cfg().MustGet(ctx, "chat.model").String(),

		// 距离度量类型
		MetricType: g.Cfg().MustGet(ctx, "vectordb.metricType", "L2").String(),
	}

	g.Log().Infof(ctx, "Shared config initialized successfully, vectordb type: %s", vectorDBType)
	return nil
}

// GetSharedConfig 获取共享配置
func GetSharedConfig() *config.Config {
	return sharedConfig
}
