package model

import (
	"context"
	"math/rand"
	"time"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/gogf/gf/v2/frame/g"
)

// LLMRetryConfig LLM重试配置
type LLMRetryConfig struct {
	MaxRetries    int           // 最大重试次数
	RetryDelay    time.Duration // 重试延迟
	ModelType     ModelType     // 模型类型（如 ModelTypeLLM）
	ExcludeModels []string      // 排除的模型ID列表
}

// DefaultLLMRetryConfig 默认LLM重试配置
func DefaultLLMRetryConfig() *LLMRetryConfig {
	return &LLMRetryConfig{
		MaxRetries:    3,                      // 默认最多重试3次
		RetryDelay:    500 * time.Millisecond, // 默认延迟500ms
		ModelType:     ModelTypeLLM,           // 默认使用LLM类型
		ExcludeModels: []string{},             // 默认不排除任何模型
	}
}

// LLMCallFunc LLM调用函数类型
// 参数：context, modelID
// 返回：result, error
type LLMCallFunc func(context.Context, string) (interface{}, error)

// RetryWithDifferentLLM 使用不同的LLM模型重试
// 如果一个模型失败，会自动切换到另一个可用的模型重试
func RetryWithDifferentLLM(ctx context.Context, config *LLMRetryConfig, callFunc LLMCallFunc) (interface{}, error) {
	if config == nil {
		config = DefaultLLMRetryConfig()
	}

	// 获取所有指定类型的模型
	allModels := Registry.GetByType(config.ModelType)
	if len(allModels) == 0 {
		return nil, errors.Newf(errors.ErrModelNotFound, "没有可用的%s类型模型", config.ModelType)
	}

	// 过滤掉排除的模型
	var availableModels []*ModelConfig
	for _, m := range allModels {
		excluded := false
		for _, excludeID := range config.ExcludeModels {
			if m.ModelID == excludeID {
				excluded = true
				break
			}
		}
		if !excluded {
			availableModels = append(availableModels, m)
		}
	}

	if len(availableModels) == 0 {
		return nil, errors.Newf(errors.ErrModelNotFound, "过滤后没有可用的%s类型模型", config.ModelType)
	}

	// 随机打乱模型顺序，避免总是使用同一个模型
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(availableModels), func(i, j int) {
		availableModels[i], availableModels[j] = availableModels[j], availableModels[i]
	})

	// 记录已尝试的模型
	triedModels := make(map[string]bool)
	var lastErr error

	// 尝试使用不同的模型
	for attempt := 0; attempt < config.MaxRetries && len(triedModels) < len(availableModels); attempt++ {
		// 选择一个未尝试过的模型
		var selectedModel *ModelConfig
		for _, m := range availableModels {
			if !triedModels[m.ModelID] {
				selectedModel = m
				break
			}
		}

		if selectedModel == nil {
			// 所有模型都已尝试过
			break
		}

		// 标记为已尝试
		triedModels[selectedModel.ModelID] = true

		g.Log().Infof(ctx, "[LLM重试] 尝试 %d/%d: 使用模型 %s (%s)",
			attempt+1, config.MaxRetries, selectedModel.Name, selectedModel.ModelID)

		// 调用LLM
		result, err := callFunc(ctx, selectedModel.ModelID)
		if err == nil {
			g.Log().Infof(ctx, "[LLM重试] 成功: 模型 %s", selectedModel.Name)
			return result, nil
		}

		// 记录错误
		lastErr = err
		g.Log().Warningf(ctx, "[LLM重试] 失败: 模型 %s, 错误: %v", selectedModel.Name, err)

		// 如果还有重试机会，等待一段时间
		if attempt < config.MaxRetries-1 && len(triedModels) < len(availableModels) {
			time.Sleep(config.RetryDelay)
		}
	}

	// 所有模型都失败了
	return nil, errors.Newf(errors.ErrLLMCallFailed, "所有%s模型都失败了，最后错误: %v", config.ModelType, lastErr)
}

// SelectRandomLLMModel 随机选择一个LLM模型
// 这是一个辅助函数，用于需要快速选择模型的场景
func SelectRandomLLMModel(ctx context.Context, modelType ModelType) (string, error) {
	models := Registry.GetByType(modelType)
	if len(models) == 0 {
		return "", errors.Newf(errors.ErrModelNotFound, "没有可用的%s类型模型", modelType)
	}

	// 随机选择一个模型
	rand.Seed(time.Now().UnixNano())
	selectedModel := models[rand.Intn(len(models))]

	g.Log().Infof(ctx, "随机选择模型: %s (%s)", selectedModel.Name, selectedModel.ModelID)
	return selectedModel.ModelID, nil
}

// SingleModelRetryConfig 单个模型重试配置
type SingleModelRetryConfig struct {
	MaxRetries int           // 最大重试次数
	RetryDelay time.Duration // 重试延迟
}

// DefaultSingleModelRetryConfig 默认单个模型重试配置
func DefaultSingleModelRetryConfig() *SingleModelRetryConfig {
	return &SingleModelRetryConfig{
		MaxRetries: 3,                      // 默认最多重试3次
		RetryDelay: 500 * time.Millisecond, // 默认延迟500ms
	}
}

// SingleModelCallFunc 单个模型调用函数类型
// 参数：context
// 返回：result, error
type SingleModelCallFunc func(context.Context) (interface{}, error)

// RetryWithSameModel 使用同一个模型重试
// 如果调用失败，会重试相同的模型，而不是切换到其他模型
func RetryWithSameModel(ctx context.Context, modelName string, config *SingleModelRetryConfig, callFunc SingleModelCallFunc) (interface{}, error) {
	if config == nil {
		config = DefaultSingleModelRetryConfig()
	}

	var lastErr error

	// 尝试调用模型
	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		if attempt > 0 {
			g.Log().Infof(ctx, "[模型重试] 尝试 %d/%d: 重试模型 %s",
				attempt+1, config.MaxRetries, modelName)
		}

		// 调用模型
		result, err := callFunc(ctx)
		if err == nil {
			if attempt > 0 {
				g.Log().Infof(ctx, "[模型重试] 成功: 模型 %s 在第 %d 次尝试成功", modelName, attempt+1)
			}
			return result, nil
		}

		// 记录错误
		lastErr = err
		g.Log().Warningf(ctx, "[模型重试] 失败: 模型 %s, 尝试 %d/%d, 错误: %v",
			modelName, attempt+1, config.MaxRetries, err)

		// 如果还有重试机会，等待一段时间
		if attempt < config.MaxRetries-1 {
			time.Sleep(config.RetryDelay)
		}
	}

	// 所有重试都失败了
	return nil, errors.Newf(errors.ErrLLMCallFailed, "模型 %s 调用失败，错误: %v", modelName, lastErr)
}
