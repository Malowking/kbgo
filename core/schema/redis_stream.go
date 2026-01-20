package schema

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis Stream 相关常量
	defaultReadTimeout    = 30 * time.Second // 读取超时时间
	defaultPollInterval   = 10 * time.Millisecond
	streamKeyPrefix       = "chat:stream:"
	streamStatusKeySuffix = ":status"
	streamStatusCompleted = "completed"
	streamStatusError     = "error"
)

var (
	// 从配置文件读取的参数
	streamTTL       time.Duration = 5 * time.Minute // Stream 过期时间
	maxStreamLength int64         = 1000            // Stream 最大长度
)

// RedisStreamReader Redis 流式数据读取器
type RedisStreamReader[T any] struct {
	streamKey  string
	statusKey  string
	lastID     string
	closed     bool
	mu         sync.Mutex
	rdb        *redis.Client
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// RedisStreamWriter Redis 流式数据写入器
type RedisStreamWriter[T any] struct {
	streamKey string
	statusKey string
	closed    bool
	mu        sync.Mutex
	rdb       *redis.Client
	ctx       context.Context
}

// 确保 RedisStreamReader 和 RedisStreamWriter 实现接口
var _ StreamReaderInterface[any] = (*RedisStreamReader[any])(nil)
var _ StreamWriterInterface[any] = (*RedisStreamWriter[any])(nil)

// RedisPipe 创建基于 Redis Stream 的流式管道
func RedisPipe[T any](ctx context.Context, streamID string) (*RedisStreamReader[T], *RedisStreamWriter[T]) {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		g.Log().Warning(ctx, "Redis client not initialized, falling back to memory stream")
		return nil, nil
	}

	streamKey := streamKeyPrefix + streamID
	statusKey := streamKey + streamStatusKeySuffix

	// 创建 reader 的独立 context（可以被取消）
	readerCtx, cancelFunc := context.WithCancel(ctx)

	reader := &RedisStreamReader[T]{
		streamKey:  streamKey,
		statusKey:  statusKey,
		lastID:     "0",
		rdb:        rdb,
		ctx:        readerCtx,
		cancelFunc: cancelFunc,
	}

	writer := &RedisStreamWriter[T]{
		streamKey: streamKey,
		statusKey: statusKey,
		rdb:       rdb,
		ctx:       ctx,
	}

	// 设置 stream 过期时间
	go func() {
		time.Sleep(streamTTL)
		if !writer.closed {
			rdb.Del(gctx.New(), streamKey, statusKey)
		}
	}()

	return reader, writer
}

// SetRedisStreamConfig 设置 Redis Stream 配置参数
func SetRedisStreamConfig(ttl time.Duration, maxLen int64) {
	if ttl > 0 {
		streamTTL = ttl
	}
	if maxLen > 0 {
		maxStreamLength = maxLen
	}
	g.Log().Infof(gctx.New(), "Redis Stream config updated: TTL=%v, MaxLength=%d", streamTTL, maxStreamLength)
}

// Recv 从 Redis Stream 读取下一个元素
func (r *RedisStreamReader[T]) Recv() (T, error) {
	var zero T

	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return zero, io.EOF
	}
	r.mu.Unlock()

	for {
		// 检查是否被取消
		select {
		case <-r.ctx.Done():
			r.mu.Lock()
			r.closed = true
			r.mu.Unlock()
			return zero, io.EOF
		default:
		}

		// 检查流状态
		status, err := r.rdb.Get(r.ctx, r.statusKey).Result()
		if err == nil {
			if status == streamStatusCompleted {
				// 尝试读取剩余消息
				result, readErr := r.readNext()
				if readErr == redis.Nil {
					// 没有更多消息，返回 EOF
					r.mu.Lock()
					r.closed = true
					r.mu.Unlock()
					return zero, io.EOF
				}
				if readErr != nil {
					return zero, readErr
				}
				return result, nil
			} else if status == streamStatusError {
				r.mu.Lock()
				r.closed = true
				r.mu.Unlock()
				return zero, errors.New(errors.ErrStreamingFailed, "stream encountered an error")
			}
		}

		// 尝试读取下一条消息
		result, err := r.readNext()
		if err == redis.Nil {
			// 没有新消息，等待一段时间后重试
			time.Sleep(defaultPollInterval)
			continue
		}
		if err != nil {
			return zero, err
		}

		return result, nil
	}
}

// readNext 从 Redis Stream 读取下一条消息
func (r *RedisStreamReader[T]) readNext() (T, error) {
	var zero T

	// 使用 XREAD 读取消息，从 lastID 之后开始读取
	streams, err := r.rdb.XRead(r.ctx, &redis.XReadArgs{
		Streams: []string{r.streamKey, r.lastID},
		Count:   1,
		Block:   100 * time.Millisecond, // 短暂阻塞，避免空轮询
	}).Result()

	if err != nil {
		return zero, err
	}

	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return zero, redis.Nil
	}

	msg := streams[0].Messages[0]
	r.lastID = msg.ID

	// 解析消息内容
	dataStr, ok := msg.Values["data"].(string)
	if !ok {
		return zero, errors.New(errors.ErrInvalidParameter, "invalid message format: missing 'data' field")
	}

	var result T
	if err := json.Unmarshal([]byte(dataStr), &result); err != nil {
		return zero, errors.Newf(errors.ErrStreamingFailed, "failed to unmarshal message: %v", err)
	}

	return result, nil
}

// Close 关闭读取器
func (r *RedisStreamReader[T]) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.closed {
		r.closed = true
		r.cancelFunc()
	}
	return nil
}

// Send 向 Redis Stream 发送一个元素
// 返回 true 表示流已关闭或发送失败
func (w *RedisStreamWriter[T]) Send(value T, err error) bool {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return true
	}
	w.mu.Unlock()

	// 如果有错误，设置错误状态
	if err != nil {
		w.rdb.Set(w.ctx, w.statusKey, streamStatusError, streamTTL)
		return false
	}

	// 序列化数据
	data, marshalErr := json.Marshal(value)
	if marshalErr != nil {
		g.Log().Errorf(w.ctx, "Failed to marshal message: %v", marshalErr)
		return true
	}

	// 使用 XADD 添加消息到 Stream
	// MAXLEN ~ 限制 Stream 最大长度，避免内存泄漏
	_, addErr := w.rdb.XAdd(w.ctx, &redis.XAddArgs{
		Stream: w.streamKey,
		MaxLen: maxStreamLength,
		Approx: true, // 使用近似裁剪，性能更好
		Values: map[string]interface{}{
			"data": string(data),
		},
	}).Result()

	if addErr != nil {
		g.Log().Errorf(w.ctx, "Failed to add message to Redis stream: %v", addErr)
		return true
	}

	return false
}

// Close 关闭写入器，并设置完成状态
func (w *RedisStreamWriter[T]) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.closed {
		w.closed = true
		// 设置流状态为已完成
		w.rdb.Set(gctx.New(), w.statusKey, streamStatusCompleted, streamTTL)
	}
	return nil
}

// IsRedisAvailable 检查 Redis 是否可用
func IsRedisAvailable() bool {
	rdb := cache.GetRedisClient()
	if rdb == nil {
		return false
	}

	ctx, cancel := context.WithTimeout(gctx.New(), 1*time.Second)
	defer cancel()

	return rdb.Ping(ctx).Err() == nil
}
