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
	rewriteModel *ChatModelConfig        // 重写模型（单例）
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
	var newRewriteModel *ChatModelConfig // 临时存储重写模型

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
						// 将 ModelConfig 转换为 LLMModelConfig
						newRewriteModel = convertToLLMModelConfig(mc)
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
func (r *ModelRegistry) GetRewriteModel() *ChatModelConfig {
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

	// 将 ModelConfig 转换为 LLMModelConfig
	r.rewriteModel = convertToLLMModelConfig(mc)
	g.Log().Infof(gctx.New(), "Rewrite model set to: %s (%s)", mc.Name, mc.ModelID)
	return nil
}

// HasRewriteModel 检查是否配置了重写模型
func (r *ModelRegistry) HasRewriteModel() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.rewriteModel != nil
}

// EmbeddingModelConfig Embedding 模型专用配置
type EmbeddingModelConfig struct {
	ModelID   string
	Name      string
	BaseURL   string
	APIKey    string
	Dimension int
}

func (e *EmbeddingModelConfig) GetDimension() int {
	return e.Dimension
}

// GetAPIKey 实现 EmbeddingConfig 接口
func (e *EmbeddingModelConfig) GetAPIKey() string {
	return e.APIKey
}

// GetBaseURL 实现 EmbeddingConfig 接口
func (e *EmbeddingModelConfig) GetBaseURL() string {
	return e.BaseURL
}

// GetEmbeddingModel 实现 EmbeddingConfig 接口
func (e *EmbeddingModelConfig) GetEmbeddingModel() string {
	return e.Name
}

// RerankerModelConfig Reranker 模型专用配置
type RerankerModelConfig struct {
	ModelID string
	Name    string
	BaseURL string
	APIKey  string
}

// ChatModelConfig 聊天模型通用配置（支持 LLM 和 Multimodal 类型）
type ChatModelConfig struct {
	ModelID             string
	Name                string
	BaseURL             string
	APIKey              string
	Enabled             bool
	Client              *openai.Client
	Type                ModelType
	Temperature         *float32
	MaxCompletionTokens *int
	TopP                *float32
	FrequencyPenalty    *float32
	PresencePenalty     *float32
	N                   *int
	Stop                []string
}

//// LLMModelConfig LLM 模型专用配置（已废弃，使用 ChatModelConfig 替代）
//// Deprecated: 使用 ChatModelConfig 替代
//type LLMModelConfig = ChatModelConfig
//
//// MultimodalModelConfig 多模态模型专用配置（已废弃，使用 ChatModelConfig 替代）
//// Deprecated: 使用 ChatModelConfig 替代
//type MultimodalModelConfig = ChatModelConfig

// GetEmbeddingModel 根据 model_id 获取 Embedding 模型配置
func (r *ModelRegistry) GetEmbeddingModel(modelID string) *EmbeddingModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mc := r.models[modelID]
	if mc == nil || mc.Type != ModelTypeEmbedding {
		return nil
	}

	// 获取维度
	dimension := 1024 // 默认值
	if mc.Extra != nil {
		if dim, exists := mc.Extra["dimension"]; exists {
			if dimInt, ok := dim.(int); ok {
				dimension = dimInt
			} else if dimFloat, ok := dim.(float64); ok {
				dimension = int(dimFloat)
			}
		}
	}

	return &EmbeddingModelConfig{
		ModelID:   mc.ModelID,
		Name:      mc.Name,
		BaseURL:   mc.BaseURL,
		APIKey:    mc.APIKey,
		Dimension: dimension,
	}
}

// GetRerankerModel 根据 model_id 获取 Reranker 模型配置
func (r *ModelRegistry) GetRerankerModel(modelID string) *RerankerModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mc := r.models[modelID]
	if mc == nil || mc.Type != ModelTypeReranker {
		return nil
	}

	return &RerankerModelConfig{
		ModelID: mc.ModelID,
		Name:    mc.Name,
		BaseURL: mc.BaseURL,
		APIKey:  mc.APIKey,
	}
}

// GetChatModel 根据 model_id 获取聊天模型配置
func (r *ModelRegistry) GetChatModel(modelID string) *ChatModelConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	mc := r.models[modelID]
	if mc == nil || (mc.Type != ModelTypeLLM && mc.Type != ModelTypeMultimodal) {
		return nil
	}

	config := &ChatModelConfig{
		ModelID: mc.ModelID,
		Name:    mc.Name,
		BaseURL: mc.BaseURL,
		APIKey:  mc.APIKey,
		Client:  mc.Client,
		Enabled: mc.Enabled,
		Type:    mc.Type,
	}

	// 从 Extra 字段提取推理参数
	if mc.Extra != nil {
		if temp, exists := mc.Extra["temperature"]; exists {
			if tempFloat, ok := temp.(float64); ok {
				tempFloat32 := float32(tempFloat)
				config.Temperature = &tempFloat32
			}
		}

		if maxTokens, exists := mc.Extra["max_completion_tokens"]; exists {
			if maxTokensInt, ok := maxTokens.(float64); ok {
				maxTokensIntVal := int(maxTokensInt)
				config.MaxCompletionTokens = &maxTokensIntVal
			} else if maxTokensInt, ok := maxTokens.(int); ok {
				config.MaxCompletionTokens = &maxTokensInt
			}
		}

		if topP, exists := mc.Extra["top_p"]; exists {
			if topPFloat, ok := topP.(float64); ok {
				topPFloat32 := float32(topPFloat)
				config.TopP = &topPFloat32
			}
		}

		if freqPenalty, exists := mc.Extra["frequency_penalty"]; exists {
			if freqPenaltyFloat, ok := freqPenalty.(float64); ok {
				freqPenaltyFloat32 := float32(freqPenaltyFloat)
				config.FrequencyPenalty = &freqPenaltyFloat32
			}
		}

		if presPenalty, exists := mc.Extra["presence_penalty"]; exists {
			if presPenaltyFloat, ok := presPenalty.(float64); ok {
				presPenaltyFloat32 := float32(presPenaltyFloat)
				config.PresencePenalty = &presPenaltyFloat32
			}
		}

		if n, exists := mc.Extra["n"]; exists {
			if nInt, ok := n.(float64); ok {
				nIntVal := int(nInt)
				config.N = &nIntVal
			} else if nInt, ok := n.(int); ok {
				config.N = &nInt
			}
		}

		if stop, exists := mc.Extra["stop"]; exists {
			if stopSlice, ok := stop.([]interface{}); ok {
				stopStrings := make([]string, 0, len(stopSlice))
				for _, s := range stopSlice {
					if str, ok := s.(string); ok {
						stopStrings = append(stopStrings, str)
					}
				}
				config.Stop = stopStrings
			} else if stopStr, ok := stop.(string); ok {
				config.Stop = []string{stopStr}
			}
		}
	}

	return config
}

//// GetLLMModel 根据 model_id 获取 LLM 模型配置
//// Deprecated: 使用 GetChatModel 替代
//func (r *ModelRegistry) GetLLMModel(modelID string) *LLMModelConfig {
//	config := r.GetChatModel(modelID)
//	if config == nil || config.Type != ModelTypeLLM {
//		return nil
//	}
//	return config
//}
//
//// GetMultimodalModel 根据 model_id 获取多模态模型配置
//// Deprecated: 使用 GetChatModel 替代
//func (r *ModelRegistry) GetMultimodalModel(modelID string) *MultimodalModelConfig {
//	config := r.GetChatModel(modelID)
//	if config == nil || config.Type != ModelTypeMultimodal {
//		return nil
//	}
//	return config
//}

// convertToLLMModelConfig 将 ModelConfig 转换为 ChatModelConfig
func convertToLLMModelConfig(mc *ModelConfig) *ChatModelConfig {
	// 直接使用 GetChatModel 的逻辑
	return Registry.GetChatModel(mc.ModelID)
}
