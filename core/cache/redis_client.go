package cache

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/redis/go-redis/v9"
)

var (
	rdb *redis.Client
)

// InitRedis 初始化Redis客户端
func InitRedis(ctx context.Context) error {
	// 从配置文件读取Redis配置
	address := g.Cfg().MustGet(ctx, "redis.address", "localhost:6379").String()
	password := g.Cfg().MustGet(ctx, "redis.password", "").String()
	db := g.Cfg().MustGet(ctx, "redis.db", 0).Int()
	maxRetries := g.Cfg().MustGet(ctx, "redis.maxRetries", 3).Int()
	poolSize := g.Cfg().MustGet(ctx, "redis.poolSize", 10).Int()
	minIdleConns := g.Cfg().MustGet(ctx, "redis.minIdleConns", 2).Int()

	rdb = redis.NewClient(&redis.Options{
		Addr:         address,
		Password:     password,
		DB:           db,
		MaxRetries:   maxRetries,
		PoolSize:     poolSize,
		MinIdleConns: minIdleConns,
	})

	// 测试连接
	if err := rdb.Ping(ctx).Err(); err != nil {
		g.Log().Errorf(ctx, "Redis connection failed: %v", err)
		return err
	}

	g.Log().Infof(ctx, "Redis initialized successfully: %s, DB: %d", address, db)
	return nil
}

// GetRedisClient 获取Redis客户端
func GetRedisClient() *redis.Client {
	return rdb
}

// CloseRedis 关闭Redis连接
func CloseRedis(ctx context.Context) error {
	if rdb != nil {
		g.Log().Info(ctx, "Closing Redis connection")
		return rdb.Close()
	}
	return nil
}

// Set 设置缓存
func Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return rdb.Set(ctx, key, value, expiration).Err()
}

// Get 获取缓存
func Get(ctx context.Context, key string) (string, error) {
	return rdb.Get(ctx, key).Result()
}

// Delete 删除缓存
func Delete(ctx context.Context, keys ...string) error {
	return rdb.Del(ctx, keys...).Err()
}

// Exists 检查key是否存在
func Exists(ctx context.Context, keys ...string) (int64, error) {
	return rdb.Exists(ctx, keys...).Result()
}

// Expire 设置过期时间
func Expire(ctx context.Context, key string, expiration time.Duration) error {
	return rdb.Expire(ctx, key, expiration).Err()
}
