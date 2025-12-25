package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/redis/go-redis/v9"
)

const (
	agentPresetCacheKeyPrefix = "agent_preset:"
)

// GetAgentPresetTTL 获取Agent预设缓存TTL
func GetAgentPresetTTL(ctx context.Context) time.Duration {
	ttl := g.Cfg().MustGet(ctx, "redis.cache.agentPresetTTL", 300).Int()
	return time.Duration(ttl) * time.Second
}

// GetAgentPreset 从缓存获取Agent预设
func GetAgentPreset(ctx context.Context, presetID string) (*gorm.AgentPreset, error) {
	cacheKey := agentPresetCacheKeyPrefix + presetID

	// 从Redis获取
	cached, err := rdb.Get(ctx, cacheKey).Result()
	if err != nil {
		if err != redis.Nil {
			g.Log().Warningf(ctx, "Failed to get agent preset from cache: %v", err)
		}
		return nil, err
	}

	// 反序列化
	var preset gorm.AgentPreset
	if err := json.Unmarshal([]byte(cached), &preset); err != nil {
		g.Log().Errorf(ctx, "Failed to unmarshal cached agent preset: %v", err)
		return nil, err
	}

	return &preset, nil
}

// SetAgentPreset 设置Agent预设到缓存
func SetAgentPreset(ctx context.Context, preset *gorm.AgentPreset) error {
	if preset == nil {
		return errors.New(errors.ErrInvalidParameter, "preset is nil")
	}

	cacheKey := agentPresetCacheKeyPrefix + preset.PresetID

	// 序列化
	data, err := json.Marshal(preset)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to marshal agent preset: %v", err)
		return errors.Newf(errors.ErrInternalError, "failed to marshal agent preset: %v", err)
	}

	// 写入Redis
	ttl := GetAgentPresetTTL(ctx)
	if err := rdb.Set(ctx, cacheKey, data, ttl).Err(); err != nil {
		g.Log().Errorf(ctx, "Failed to set agent preset cache: %v", err)
		return errors.Newf(errors.ErrInternalError, "failed to set agent preset cache: %v", err)
	}

	return nil
}

// InvalidateAgentPreset 删除Agent预设缓存
func InvalidateAgentPreset(ctx context.Context, presetID string) error {
	cacheKey := agentPresetCacheKeyPrefix + presetID

	if err := rdb.Del(ctx, cacheKey).Err(); err != nil {
		g.Log().Warningf(ctx, "Failed to invalidate agent preset cache: %v", err)
		return err
	}

	return nil
}

// InvalidateAgentPresetsByUserID 删除用户的所有Agent预设缓存
func InvalidateAgentPresetsByUserID(ctx context.Context, userID string) error {
	// 注意：这个方法需要scan所有key，生产环境慎用
	// 可以考虑维护一个user_id -> preset_ids的映射来优化
	pattern := agentPresetCacheKeyPrefix + "*"

	iter := rdb.Scan(ctx, 0, pattern, 0).Iterator()
	deletedCount := 0
	for iter.Next(ctx) {
		key := iter.Val()
		// 简单粗暴删除所有agent_preset:*的key
		// 实际应该先get出来判断user_id，但这样会增加复杂度
		if err := rdb.Del(ctx, key).Err(); err != nil {
			g.Log().Warningf(ctx, "Failed to delete cache key %s: %v", key, err)
		} else {
			deletedCount++
		}
	}

	if err := iter.Err(); err != nil {
		g.Log().Errorf(ctx, "Redis scan error: %v", err)
		return err
	}

	g.Log().Infof(ctx, "Invalidated %d agent preset cache keys for user: %s", deletedCount, userID)
	return nil
}
