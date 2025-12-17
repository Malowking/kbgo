package model

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

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

// ModelConfig 模型配置（内存缓存）
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

// ModelRegistry 全局模型注册表（内存缓存）
type ModelRegistry struct {
	mu     sync.RWMutex
	models map[string]*ModelConfig // key = model_id (UUID)
}

// Registry 全局单例
var Registry = &ModelRegistry{
	models: make(map[string]*ModelConfig),
}

// Get 获取模型配置（并发安全读取）
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
	return result
}

// Reload 从数据库重新加载模型配置（热更新）
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
	r.mu.Unlock()

	g.Log().Infof(ctx, "Model registry reloaded successfully, total models: %d", len(newMap))
	return nil
}

// Count 返回当前加载的模型数量
func (r *ModelRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.models)
}
