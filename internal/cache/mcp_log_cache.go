package cache

import (
	"context"
	"encoding/json"
	"fmt"
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
	mcpCallLogKeyPrefix      = "mcp_log:"
	convMCPLogListPrefix     = "conv_mcp_logs:"
	mcpRegistryLogListPrefix = "mcp_registry_logs:"

	// 默认缓存过期时间
	defaultMCPLogTTL = 48 * time.Hour // MCP调用日志缓存48小时

	// 刷盘配置
	defaultMCPFlushInterval = 30 * time.Second // 30秒刷盘一次
	defaultMCPBatchSize     = 50               // 每批次刷盘50条
)

// MCPCallLogCache MCP调用日志缓存层
type MCPCallLogCache struct {
	rdb          *redis.Client
	flushTicker  *time.Ticker
	pendingQueue chan *gormModel.MCPCallLog
	wg           sync.WaitGroup
	ctx          context.Context
	cancel       context.CancelFunc
}

var (
	mcpLogCache     *MCPCallLogCache
	mcpLogCacheOnce sync.Once
)

// InitMCPCallLogCache 初始化MCP调用日志缓存
func InitMCPCallLogCache(ctx context.Context) error {
	var initErr error
	mcpLogCacheOnce.Do(func() {
		rdb := cache.GetRedisClient()
		if rdb == nil {
			initErr = fmt.Errorf("Redis客户端未初始化")
			return
		}

		cctx, cancel := context.WithCancel(ctx)
		mcpLogCache = &MCPCallLogCache{
			rdb:          rdb,
			flushTicker:  time.NewTicker(defaultMCPFlushInterval),
			pendingQueue: make(chan *gormModel.MCPCallLog, 1000),
			ctx:          cctx,
			cancel:       cancel,
		}

		// 启动刷盘协程
		mcpLogCache.wg.Add(1)
		go mcpLogCache.flushWorker()

		g.Log().Info(ctx, "MCP调用日志缓存层初始化成功")
	})

	return initErr
}

// GetMCPCallLogCache 获取MCP调用日志缓存实例
func GetMCPCallLogCache() *MCPCallLogCache {
	return mcpLogCache
}

// SaveMCPCallLog 保存MCP调用日志到缓存（异步刷盘到数据库）
func (mc *MCPCallLogCache) SaveMCPCallLog(ctx context.Context, log *gormModel.MCPCallLog) error {
	// 1. 先写入Redis缓存
	if err := mc.saveToCache(ctx, log); err != nil {
		g.Log().Errorf(ctx, "写入MCP日志到Redis缓存失败: %v", err)
		// 缓存失败时直接写数据库
		return mc.saveToDatabase(ctx, log)
	}

	// 2. 将日志加入刷盘队列
	select {
	case mc.pendingQueue <- log:
		// 成功加入队列
	default:
		// 队列满了，直接写数据库
		g.Log().Warning(ctx, "MCP日志刷盘队列已满，直接写入数据库")
		return mc.saveToDatabase(ctx, log)
	}

	return nil
}

// saveToCache 保存MCP日志到Redis
func (mc *MCPCallLogCache) saveToCache(ctx context.Context, log *gormModel.MCPCallLog) error {
	// 1. 保存日志主体
	logKey := fmt.Sprintf("%s%s", mcpCallLogKeyPrefix, log.ID)
	logJSON, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("序列化MCP日志失败: %w", err)
	}
	if err := mc.rdb.Set(ctx, logKey, logJSON, defaultMCPLogTTL).Err(); err != nil {
		return fmt.Errorf("保存MCP日志到Redis失败: %w", err)
	}

	// 2. 将日志ID添加到会话的日志列表中（使用sorted set，按创建时间排序）
	if log.ConversationID != "" {
		convListKey := fmt.Sprintf("%s%s", convMCPLogListPrefix, log.ConversationID)
		score := float64(log.CreateTime.Unix())
		if err := mc.rdb.ZAdd(ctx, convListKey, redis.Z{
			Score:  score,
			Member: log.ID,
		}).Err(); err != nil {
			g.Log().Errorf(ctx, "添加MCP日志到会话列表失败: %v", err)
		}
		// 设置会话列表的过期时间
		mc.rdb.Expire(ctx, convListKey, defaultMCPLogTTL)
	}

	// 3. 将日志ID添加到MCP服务的日志列表中
	if log.MCPRegistryID != "" {
		registryListKey := fmt.Sprintf("%s%s", mcpRegistryLogListPrefix, log.MCPRegistryID)
		score := float64(log.CreateTime.Unix())
		if err := mc.rdb.ZAdd(ctx, registryListKey, redis.Z{
			Score:  score,
			Member: log.ID,
		}).Err(); err != nil {
			g.Log().Errorf(ctx, "添加MCP日志到服务列表失败: %v", err)
		}
		// 设置服务列表的过期时间
		mc.rdb.Expire(ctx, registryListKey, defaultMCPLogTTL)
	}

	return nil
}

// saveToDatabase 保存MCP日志到数据库
func (mc *MCPCallLogCache) saveToDatabase(ctx context.Context, log *gormModel.MCPCallLog) error {
	return dao.MCPCallLog.Create(ctx, log)
}

// GetMCPCallLog 获取MCP调用日志（优先从缓存读取）
func (mc *MCPCallLogCache) GetMCPCallLog(ctx context.Context, id string) (*gormModel.MCPCallLog, error) {
	// 1. 先从Redis缓存读取
	logKey := fmt.Sprintf("%s%s", mcpCallLogKeyPrefix, id)
	logJSON, err := mc.rdb.Get(ctx, logKey).Result()
	if err == nil {
		// 缓存命中
		var log gormModel.MCPCallLog
		if err := json.Unmarshal([]byte(logJSON), &log); err == nil {
			return &log, nil
		}
	} else if err != redis.Nil {
		g.Log().Errorf(ctx, "从Redis读取MCP日志失败: %v", err)
	}

	// 2. 缓存未命中，从数据库读取
	log, err := dao.MCPCallLog.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if log == nil {
		return nil, nil
	}

	// 3. 回写缓存
	go mc.saveToCache(context.Background(), log)

	return log, nil
}

// GetMCPCallLogsByConvID 获取会话的MCP调用日志列表（优先从缓存读取）
func (mc *MCPCallLogCache) GetMCPCallLogsByConvID(ctx context.Context, convID string, page, pageSize int) ([]*gormModel.MCPCallLog, int64, error) {
	convListKey := fmt.Sprintf("%s%s", convMCPLogListPrefix, convID)

	// 1. 检查缓存是否存在
	exists, err := mc.rdb.Exists(ctx, convListKey).Result()
	if err == nil && exists > 0 {
		// 从缓存读取
		total, err := mc.rdb.ZCard(ctx, convListKey).Result()
		if err != nil {
			g.Log().Errorf(ctx, "获取会话MCP日志总数失败: %v", err)
		}

		// 分页读取日志ID（倒序，最新的在前）
		offset := int64((page - 1) * pageSize)
		start := -offset - int64(pageSize)
		end := -offset - 1
		if start < -total {
			start = 0
			end = int64(pageSize) - 1
		}

		logIDs, err := mc.rdb.ZRevRange(ctx, convListKey, start, end).Result()
		if err != nil {
			g.Log().Errorf(ctx, "获取会话MCP日志列表失败: %v", err)
		} else {
			// 批量获取日志
			var logs []*gormModel.MCPCallLog
			for _, logID := range logIDs {
				log, err := mc.GetMCPCallLog(ctx, logID)
				if err == nil && log != nil {
					logs = append(logs, log)
				}
			}
			if len(logs) > 0 {
				return logs, total, nil
			}
		}
	}

	// 2. 缓存未命中，从数据库读取
	logs, total, err := dao.MCPCallLog.ListByConversationID(ctx, convID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// 3. 回写缓存（异步）
	go func() {
		for _, log := range logs {
			mc.saveToCache(context.Background(), log)
		}
	}()

	return logs, total, nil
}

// GetMCPCallLogsByRegistryID 获取MCP服务的调用日志列表（优先从缓存读取）
func (mc *MCPCallLogCache) GetMCPCallLogsByRegistryID(ctx context.Context, registryID string, page, pageSize int) ([]*gormModel.MCPCallLog, int64, error) {
	registryListKey := fmt.Sprintf("%s%s", mcpRegistryLogListPrefix, registryID)

	// 1. 检查缓存是否存在
	exists, err := mc.rdb.Exists(ctx, registryListKey).Result()
	if err == nil && exists > 0 {
		// 从缓存读取
		total, err := mc.rdb.ZCard(ctx, registryListKey).Result()
		if err != nil {
			g.Log().Errorf(ctx, "获取MCP服务日志总数失败: %v", err)
		}

		// 分页读取日志ID（倒序，最新的在前）
		offset := int64((page - 1) * pageSize)
		start := -offset - int64(pageSize)
		end := -offset - 1
		if start < -total {
			start = 0
			end = int64(pageSize) - 1
		}

		logIDs, err := mc.rdb.ZRevRange(ctx, registryListKey, start, end).Result()
		if err != nil {
			g.Log().Errorf(ctx, "获取MCP服务日志列表失败: %v", err)
		} else {
			// 批量获取日志
			var logs []*gormModel.MCPCallLog
			for _, logID := range logIDs {
				log, err := mc.GetMCPCallLog(ctx, logID)
				if err == nil && log != nil {
					logs = append(logs, log)
				}
			}
			if len(logs) > 0 {
				return logs, total, nil
			}
		}
	}

	// 2. 缓存未命中，从数据库读取
	logs, total, err := dao.MCPCallLog.ListByMCPRegistry(ctx, registryID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	// 3. 回写缓存（异步）
	go func() {
		for _, log := range logs {
			mc.saveToCache(context.Background(), log)
		}
	}()

	return logs, total, nil
}

// flushWorker 定期刷盘协程
func (mc *MCPCallLogCache) flushWorker() {
	defer mc.wg.Done()

	batch := make([]*gormModel.MCPCallLog, 0, defaultMCPBatchSize)

	for {
		select {
		case <-mc.ctx.Done():
			// 退出前刷盘所有剩余数据
			mc.flushBatch(batch)
			for len(mc.pendingQueue) > 0 {
				log := <-mc.pendingQueue
				if err := mc.saveToDatabase(context.Background(), log); err != nil {
					g.Log().Errorf(context.Background(), "刷盘MCP日志失败: %v", err)
				}
			}
			return

		case <-mc.flushTicker.C:
			// 定时刷盘
			if len(batch) > 0 {
				mc.flushBatch(batch)
				batch = make([]*gormModel.MCPCallLog, 0, defaultMCPBatchSize)
			}

		case log := <-mc.pendingQueue:
			batch = append(batch, log)
			if len(batch) >= defaultMCPBatchSize {
				// 批次满了，立即刷盘
				mc.flushBatch(batch)
				batch = make([]*gormModel.MCPCallLog, 0, defaultMCPBatchSize)
			}
		}
	}
}

// flushBatch 批量刷盘
func (mc *MCPCallLogCache) flushBatch(batch []*gormModel.MCPCallLog) {
	if len(batch) == 0 {
		return
	}

	ctx := context.Background()
	successCount := 0
	failCount := 0

	for _, log := range batch {
		if err := mc.saveToDatabase(ctx, log); err != nil {
			g.Log().Errorf(ctx, "刷盘MCP日志失败 (id=%s): %v", log.ID, err)
			failCount++
		} else {
			successCount++
		}
	}

	if successCount > 0 {
		g.Log().Infof(ctx, "刷盘MCP日志完成: 成功=%d, 失败=%d", successCount, failCount)
	}
}

// Close 关闭缓存层
func (mc *MCPCallLogCache) Close() {
	if mc.cancel != nil {
		mc.cancel()
	}
	mc.wg.Wait()
	if mc.flushTicker != nil {
		mc.flushTicker.Stop()
	}
}
