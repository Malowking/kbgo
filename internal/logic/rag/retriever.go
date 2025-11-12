package rag

import (
	"github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/core/config"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

var ragSvr = &core.Rag{}

func init() {
	ctx := gctx.New()
	// 检查Milvus配置是否存在
	milvusAddress := g.Cfg().MustGet(ctx, "milvus.address", "").String()
	if milvusAddress == "" {
		g.Log().Fatalf(ctx, "Milvus configuration is missing: milvus.address is required")
		return
	}

	//创建Milvus客户端
	milvusDatabase := g.Cfg().MustGet(ctx, "milvus.database").String()
	MilvusClient, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: milvusAddress,
		DBName:  milvusDatabase,
	})
	if err != nil {
		g.Log().Fatalf(ctx, "failed to create Milvus client: %v\n", err)
		return
	}

	ragSvr, err = core.New(ctx, &config.Config{
		Client:         MilvusClient,
		Database:       g.Cfg().MustGet(ctx, "milvus.database").String(),
		APIKey:         g.Cfg().MustGet(ctx, "embedding.apiKey").String(),
		BaseURL:        g.Cfg().MustGet(ctx, "embedding.baseURL").String(),
		EmbeddingModel: g.Cfg().MustGet(ctx, "embedding.model").String(),
		ChatModel:      g.Cfg().MustGet(ctx, "chat.model").String(),
	})
	if err != nil {
		g.Log().Fatalf(ctx, "New of rag failed, err=%v", err)
		return
	}
}

func GetRagSvr() *core.Rag {
	return ragSvr
}
