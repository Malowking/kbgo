package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// CreateStreamPipe 根据配置创建流式传输管道
// 优先使用 Redis Stream (如果配置启用)，否则使用内存 channel
// 返回接口类型，支持两种实现的无缝切换
func CreateStreamPipe(ctx context.Context, convID string) (schema.StreamReaderInterface[*schema.Message], schema.StreamWriterInterface[*schema.Message]) {
	// 从配置读取流式响应配置
	useRedis := g.Cfg().MustGet(ctx, "streaming.useRedis", false).Bool()
	memoryBufferSize := g.Cfg().MustGet(ctx, "streaming.memoryBufferSize", 100).Int()
	redisBufferSize := g.Cfg().MustGet(ctx, "streaming.redisBufferSize", 1000).Int64()
	redisStreamTTL := g.Cfg().MustGet(ctx, "streaming.redisStreamTTL", 300).Int()

	// 如果启用 Redis Stream 且 Redis 可用
	if useRedis && schema.IsRedisAvailable() {
		// 设置 Redis Stream 配置
		schema.SetRedisStreamConfig(
			time.Duration(redisStreamTTL)*time.Second,
			redisBufferSize,
		)

		// 生成唯一 streamID
		streamID := fmt.Sprintf("%s:%d", convID, time.Now().UnixNano())
		redisReader, redisWriter := schema.RedisPipe[*schema.Message](ctx, streamID)

		if redisReader != nil && redisWriter != nil {
			g.Log().Infof(ctx, "Using Redis Stream for streaming response: %s (maxLen=%d, TTL=%ds)",
				streamID, redisBufferSize, redisStreamTTL)

			// 直接返回 Redis Stream 的 reader 和 writer（它们实现了接口）
			return redisReader, redisWriter
		}

		g.Log().Warning(ctx, "Failed to create Redis stream, falling back to memory channel")
	}

	// 使用内存 channel
	g.Log().Debugf(ctx, "Using memory channel for streaming response (bufferSize=%d)", memoryBufferSize)
	memReader, memWriter := schema.Pipe[*schema.Message](memoryBufferSize)
	return memReader, memWriter
}
