package rag

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

// RetrieverConfig 检索默认参数配置
type RetrieverConfig struct {
	EnableRewrite   bool   `json:"enableRewrite" yaml:"enableRewrite"`     // 是否启用查询重写（默认 false）
	RewriteAttempts int    `json:"rewriteAttempts" yaml:"rewriteAttempts"` // 查询重写尝试次数（默认 3）
	RetrieveMode    string `json:"retrieveMode" yaml:"retrieveMode"`       // 检索模式: milvus/rerank/rrf（默认 rerank）
}

var retrieverConfig *RetrieverConfig

// init 初始化检索配置
func InitRetrieverConfig() {
	ctx := gctx.New()

	// 加载检索配置
	var cfg RetrieverConfig
	err := g.Cfg().MustGet(ctx, "retriever").Scan(&cfg)
	if err != nil {
		g.Log().Warningf(ctx, "load retriever config failed, using defaults, err=%v", err)
		// 使用默认值
		cfg = RetrieverConfig{
			EnableRewrite:   false,
			RewriteAttempts: 3,
			RetrieveMode:    "rerank",
		}
	}

	retrieverConfig = &cfg
	g.Log().Infof(ctx, "retriever config loaded: EnableRewrite=%v, RewriteAttempts=%d, RetrieveMode=%s",
		cfg.EnableRewrite, cfg.RewriteAttempts, cfg.RetrieveMode)
}

// GetRetrieverConfig 获取检索配置
func GetRetrieverConfig() *RetrieverConfig {
	if retrieverConfig == nil {
		return &RetrieverConfig{
			EnableRewrite:   false,
			RewriteAttempts: 3,
			RetrieveMode:    "rerank",
		}
	}
	return retrieverConfig
}
