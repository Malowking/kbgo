package model

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gogf/gf/v2/os/gctx"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/sashabaranov/go-openai"
	gormdb "gorm.io/gorm"
)

// ModelType 模型类型
type ModelType string

const (
	ModelTypeLLM        ModelType = "llm"        // 大语言模型
	ModelTypeEmbedding  ModelType = "embedding"  // 向量化模型
	ModelTypeReranker   ModelType = "reranker"   // 重排序模型
	ModelTypeMultimodal ModelType = "multimodal" // 多模态模型
	ModelTypeImage      ModelType = "image"      // 文生图模型
	ModelTypeVideo      ModelType = "video"      // 文生视频模型
	ModelTypeAudio      ModelType = "audio"      // 文生音频模型
)

// ModelConfig 模型配置
type ModelConfig struct {
	ModelID  string         `json:"model_id"` // UUID
	Name     string         `json:"name"`     // 模型名称
	Version  string         `json:"version"`  // 模型版本
	Type     ModelType      `json:"type"`     // 模型类型
	Provider string         `json:"provider"` // 提供商
	BaseURL  string         `json:"base_url"` // API Base URL
	APIKey   string         `json:"api_key"`  // API Key
	Enabled  bool           `json:"enabled"`  // 是否启用
	Extra    map[string]any `json:"extra"`    // 扩展配置
	Client   *openai.Client `json:"-"`        // OpenAI 客户端（不序列化）
}

// ModelRegistry 全局模型注册表
type ModelRegistry struct {
	mu           sync.RWMutex
	models       map[string]*ModelConfig // key = model_id (UUID)
	rewriteModel *ModelConfig            // 重写模型（单例）
}

// Registry 全局单例
var Registry = &ModelRegistry{
	models:       make(map[string]*ModelConfig),
	rewriteModel: nil, // 初始为空，等待用户配置
}

// Get 获取模型配置
func (r *ModelRegistry) Get(modelID string) *ModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.models[modelID]
}

// GetByType 根据类型获取所有模型
func (r *ModelRegistry) GetByType(modelType ModelType) []*ModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*ModelConfig
	for _, mc := range r.models {
		if mc.Type == modelType {
			result = append(result, mc)
		}
	}

	// 按模型名称排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// List 列出所有模型
func (r *ModelRegistry) List() []*ModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*ModelConfig, 0, len(r.models))
	for _, mc := range r.models {
		result = append(result, mc)
	}

	// 按模型名称排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// Reload 从数据库重新加载模型配置
func (r *ModelRegistry) Reload(ctx context.Context, db *gormdb.DB) error {
	// 查询所有模型（包括禁用的，以便前端显示完整列表）
	var models []gorm.AIModel
	result := db.Find(&models)
	if result.Error != nil {
		g.Log().Errorf(ctx, "Failed to query model table: %v", result.Error)
		return result.Error
	}

	// 构建新的模型映射
	newMap := make(map[string]*ModelConfig)
	var newRewriteModel *ModelConfig // 临时存储重写模型

	for _, m := range models {
		mc := &ModelConfig{
			ModelID:  m.ModelID,
			Name:     m.ModelName,
			Version:  m.Version,
			Type:     ModelType(m.ModelType),
			Provider: m.Provider,
			BaseURL:  m.BaseURL,
			APIKey:   m.APIKey,
			Enabled:  m.Enabled, // 保存启用状态
		}

		// 解析 extra JSON
		if m.Extra != "" {
			var extra map[string]any
			if err := json.Unmarshal([]byte(m.Extra), &extra); err == nil {
				mc.Extra = extra

				// 检查是否是重写模型
				if isRewrite, ok := extra["is_rewrite"].(bool); ok && isRewrite {
					// 验证重写模型必须是启用的 LLM 类型
					if m.Enabled && mc.Type == ModelTypeLLM {
						newRewriteModel = mc
						g.Log().Infof(ctx, "Found rewrite model in database: %s (%s)", mc.Name, mc.ModelID)
					} else {
						g.Log().Warningf(ctx, "Invalid rewrite model config: %s (%s), type=%s, enabled=%v",
							mc.Name, mc.ModelID, mc.Type, m.Enabled)
					}
				}
			}
		}

		// 只为启用的模型创建 OpenAI 客户端
		if m.Enabled {
			// 创建带超时的 HTTP 客户端
			httpClient := &http.Client{
				Timeout: 300 * time.Second, // 总超时时间 5 分钟
				Transport: &http.Transport{
					DialContext: (&net.Dialer{
						Timeout:   10 * time.Second, // 连接超时 10 秒
						KeepAlive: 30 * time.Second,
					}).DialContext,
					TLSHandshakeTimeout:   10 * time.Second, // TLS 握手超时
					ResponseHeaderTimeout: 30 * time.Second, // 响应头超时（必须在 30 秒内开始返回数据）
					IdleConnTimeout:       90 * time.Second,
					MaxIdleConns:          100,
					MaxIdleConnsPerHost:   10,
				},
			}

			// 创建 OpenAI 客户端
			config := openai.DefaultConfig(m.APIKey)
			config.BaseURL = m.BaseURL
			config.HTTPClient = httpClient // 设置自定义 HTTP 客户端
			mc.Client = openai.NewClientWithConfig(config)
		}

		newMap[m.ModelID] = mc
	}

	// 原子替换（所有旧请求继续使用旧缓存，新请求使用新缓存）
	r.mu.Lock()
	r.models = newMap
	r.rewriteModel = newRewriteModel // 同步更新重写模型
	r.mu.Unlock()

	if newRewriteModel != nil {
		g.Log().Infof(ctx, "Model registry reloaded successfully, total models: %d, rewrite model: %s",
			len(newMap), newRewriteModel.Name)
	} else {
		g.Log().Infof(ctx, "Model registry reloaded successfully, total models: %d, no rewrite model configured",
			len(newMap))
	}
	return nil
}

// Count 返回当前加载的模型数量
func (r *ModelRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}

// GetRewriteModel 获取重写模型配置
func (r *ModelRegistry) GetRewriteModel() *ModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rewriteModel
}

// SetRewriteModel 设置重写模型
func (r *ModelRegistry) SetRewriteModel(modelID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 如果 modelID 为空，清除重写模型
	if modelID == "" {
		r.rewriteModel = nil
		return nil
	}

	// 检查模型是否存在
	mc, exists := r.models[modelID]
	if !exists {
		return errors.Newf(errors.ErrModelNotFound, "model not found: %s", modelID)
	}

	// 检查模型是否启用
	if !mc.Enabled {
		return errors.New(errors.ErrModelConfigInvalid, "cannot set disabled model as rewrite model")
	}

	// 检查模型类型是否为 LLM
	if mc.Type != ModelTypeLLM {
		return errors.New(errors.ErrModelConfigInvalid, "rewrite model must be LLM type")
	}

	// 设置重写模型
	r.rewriteModel = mc
	g.Log().Infof(gctx.New(), "Rewrite model set to: %s (%s)", mc.Name, mc.ModelID)
	return nil
}

// HasRewriteModel 检查是否配置了重写模型
func (r *ModelRegistry) HasRewriteModel() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rewriteModel != nil
}
