package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/os/gctx"
	"sync"
	"time"

	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis key前缀
	messageKeyPrefix        = "msg:"
	messageContentKeyPrefix = "msg_content:"
	convMessageListPrefix   = "conv_msgs:"

	// 默认缓存过期时间
	defaultMessageTTL = 24 * time.Hour // 消息缓存24小时

	// 刷盘配置
	defaultFlushInterval = 30 * time.Second // 30秒刷盘一次
	defaultBatchSize     = 100              // 每批次刷盘100条
)

// MessageCache 消息缓存层
type MessageCache struct {
	rdb          *redis.Client
	flushTicker  *time.Ticker
	pendingQueue chan *PendingMessage
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

// PendingMessage 待刷盘的消息
type PendingMessage struct {
	Message  *gormModel.Message
	Contents []*gormModel.MessageContent
}

var (
	messageCache     *MessageCache
	messageCacheOnce sync.Once
)

// InitMessageCache 初始化消息缓存
func InitMessageCache(ctx context.Context) error {
	var initErr error
	messageCacheOnce.Do(func() {
		rdb := cache.GetRedisClient()
		if rdb == nil {
			initErr = fmt.Errorf("Redis客户端未初始化")
			return
		}

		cctx, cancel := context.WithCancel(ctx)
		messageCache = &MessageCache{
			rdb:          rdb,
			flushTicker:  time.NewTicker(defaultFlushInterval),
			pendingQueue: make(chan *PendingMessage, 1000),
			ctx:          cctx,
			cancel:       cancel,
		}

		// 启动刷盘协程
		messageCache.wg.Add(1)
		go messageCache.flushWorker()

		g.Log().Info(ctx, "消息缓存层初始化成功")
	})

	return initErr
}

// GetMessageCache 获取消息缓存实例
func GetMessageCache() *MessageCache {
	return messageCache
}

// SaveMessage 保存消息到缓存
func (mc *MessageCache) SaveMessage(ctx context.Context, message *gormModel.Message, contents []*gormModel.MessageContent) error {
	// 1. 先写入Redis缓存
	if err := mc.saveToCache(ctx, message, contents); err != nil {
		g.Log().Errorf(ctx, "写入Redis缓存失败: %v", err)
		// 缓存失败时直接写数据库
		return mc.saveToDatabase(ctx, message, contents)
	}

	// 2. 将消息加入刷盘队列
	select {
	case mc.pendingQueue <- &PendingMessage{
		Message:  message,
		Contents: contents,
	}:
		// 成功加入队列
	default:
		// 队列满了，直接写数据库
		g.Log().Warning(ctx, "刷盘队列已满，直接写入数据库")
		return mc.saveToDatabase(ctx, message, contents)
	}

	return nil
}

// saveToCache 保存消息到Redis
func (mc *MessageCache) saveToCache(ctx context.Context, message *gormModel.Message, contents []*gormModel.MessageContent) error {
	// 1. 保存消息主体
	msgKey := fmt.Sprintf("%s%s", messageKeyPrefix, message.MsgID)
	msgJSON, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}
	if err := mc.rdb.Set(ctx, msgKey, msgJSON, defaultMessageTTL).Err(); err != nil {
		return fmt.Errorf("保存消息到Redis失败: %w", err)
	}

	// 2. 保存消息内容块
	for _, content := range contents {
		contentKey := fmt.Sprintf("%s%s:%d", messageContentKeyPrefix, message.MsgID, content.SortOrder)
		contentJSON, err := json.Marshal(content)
		if err != nil {
			g.Log().Errorf(ctx, "序列化消息内容块失败: %v", err)
			continue
		}
		if err := mc.rdb.Set(ctx, contentKey, contentJSON, defaultMessageTTL).Err(); err != nil {
			g.Log().Errorf(ctx, "保存消息内容块到Redis失败: %v", err)
		}
	}

	// 3. 将消息ID添加到会话的消息列表中
	convListKey := fmt.Sprintf("%s%s", convMessageListPrefix, message.ConvID)
	score := float64(message.CreateTime.Unix())
	if err := mc.rdb.ZAdd(ctx, convListKey, redis.Z{
		Score:  score,
		Member: message.MsgID,
	}).Err(); err != nil {
		g.Log().Errorf(ctx, "添加消息到会话列表失败: %v", err)
	}
	// 设置会话列表的过期时间
	mc.rdb.Expire(ctx, convListKey, defaultMessageTTL)

	return nil
}

// saveToDatabase 保存消息到数据库
func (mc *MessageCache) saveToDatabase(ctx context.Context, message *gormModel.Message, contents []*gormModel.MessageContent) error {
	return dao.Message.CreateWithContents(ctx, message, contents)
}

// GetMessage 获取消息
func (mc *MessageCache) GetMessage(ctx context.Context, msgID string) (*gormModel.Message, []*gormModel.MessageContent, error) {
	// 1. 先从Redis缓存读取
	msgKey := fmt.Sprintf("%s%s", messageKeyPrefix, msgID)
	msgJSON, err := mc.rdb.Get(ctx, msgKey).Result()
	if err == nil {
		// 缓存命中
		var message gormModel.Message
		if err := json.Unmarshal([]byte(msgJSON), &message); err == nil {
			// 读取内容块
			contents, _ := mc.getMessageContentsFromCache(ctx, msgID)
			return &message, contents, nil
		}
	} else if err != redis.Nil {
		g.Log().Errorf(ctx, "从Redis读取消息失败: %v", err)
	}

	// 2. 缓存未命中，从数据库读取
	message, err := dao.Message.GetByMsgID(ctx, msgID)
	if err != nil {
		return nil, nil, err
	}
	if message == nil {
		return nil, nil, nil
	}

	contents, err := dao.MessageContent.ListByMsgID(ctx, msgID)
	if err != nil {
		return nil, nil, err
	}

	// 3. 回写缓存
	go mc.saveToCache(gctx.New(), message, contents)

	return message, contents, nil
}

// getMessageContentsFromCache 从缓存获取消息内容块
func (mc *MessageCache) getMessageContentsFromCache(ctx context.Context, msgID string) ([]*gormModel.MessageContent, error) {
	// 使用SCAN查找该消息的所有内容块
	pattern := fmt.Sprintf("%s%s:*", messageContentKeyPrefix, msgID)
	var contents []*gormModel.MessageContent

	iter := mc.rdb.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		contentJSON, err := mc.rdb.Get(ctx, iter.Val()).Result()
		if err != nil {
			continue
		}

		var content gormModel.MessageContent
		if err := json.Unmarshal([]byte(contentJSON), &content); err != nil {
			continue
		}
		contents = append(contents, &content)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	return contents, nil
}

// GetMessagesByConvID 获取会话的消息列表
func (mc *MessageCache) GetMessagesByConvID(ctx context.Context, convID string, page, pageSize int) ([]*gormModel.Message, int64, error) {
	convListKey := fmt.Sprintf("%s%s", convMessageListPrefix, convID)

	// 1. 检查缓存是否存在
	exists, err := mc.rdb.Exists(ctx, convListKey).Result()
	if err == nil && exists > 0 {
		// 从缓存读取
		total, err := mc.rdb.ZCard(ctx, convListKey).Result()
		if err != nil {
			g.Log().Errorf(ctx, "获取会话消息总数失败: %v", err)
		}

		// 分页读取消息ID
		offset := int64((page - 1) * pageSize)
		msgIDs, err := mc.rdb.ZRange(ctx, convListKey, offset, offset+int64(pageSize)-1).Result()
		if err != nil {
			g.Log().Errorf(ctx, "获取会话消息列表失败: %v", err)
		} else {
			// 批量获取消息
			var messages []*gormModel.Message
			for _, msgID := range msgIDs {
				msg, _, err := mc.GetMessage(ctx, msgID)
				if err == nil && msg != nil {
					messages = append(messages, msg)
				}
			}
			if len(messages) > 0 {
				return messages, total, nil
			}
		}
	}

	// 2. 缓存未命中，从数据库读取
	messages, total, err := dao.Message.ListByConvID(ctx, convID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// 3. 回写缓存
	go func() {
		for _, msg := range messages {
			contents, _ := dao.MessageContent.ListByMsgID(gctx.New(), msg.MsgID)
			mc.saveToCache(gctx.New(), msg, contents)
		}
	}()

	return messages, total, nil
}

// flushWorker 定期刷盘协程
func (mc *MessageCache) flushWorker() {
	defer mc.wg.Done()

	batch := make([]*PendingMessage, 0, defaultBatchSize)

	for {
		select {
		case <-mc.ctx.Done():
			// 退出前刷盘所有剩余数据
			mc.flushBatch(batch)
			for len(mc.pendingQueue) > 0 {
				msg := <-mc.pendingQueue
				if err := mc.saveToDatabase(gctx.New(), msg.Message, msg.Contents); err != nil {
					g.Log().Errorf(gctx.New(), "刷盘消息失败: %v", err)
				}
			}
			return

		case <-mc.flushTicker.C:
			// 定时刷盘
			if len(batch) > 0 {
				mc.flushBatch(batch)
				batch = make([]*PendingMessage, 0, defaultBatchSize)
			}

		case msg := <-mc.pendingQueue:
			batch = append(batch, msg)
			if len(batch) >= defaultBatchSize {
				// 批次满了，立即刷盘
				mc.flushBatch(batch)
				batch = make([]*PendingMessage, 0, defaultBatchSize)
			}
		}
	}
}

// flushBatch 批量刷盘
func (mc *MessageCache) flushBatch(batch []*PendingMessage) {
	if len(batch) == 0 {
		return
	}

	ctx := gctx.New()
	successCount := 0
	failCount := 0

	for _, pending := range batch {
		if err := mc.saveToDatabase(ctx, pending.Message, pending.Contents); err != nil {
			g.Log().Errorf(ctx, "刷盘消息失败 (msgID=%s): %v", pending.Message.MsgID, err)
			failCount++
		} else {
			successCount++
		}
	}

	if successCount > 0 {
		g.Log().Infof(ctx, "刷盘消息完成: 成功=%d, 失败=%d", successCount, failCount)
	}
}

// Close 关闭缓存层
func (mc *MessageCache) Close() {
	if mc.cancel != nil {
		mc.cancel()
	}
	mc.wg.Wait()
	if mc.flushTicker != nil {
		mc.flushTicker.Stop()
	}
}
