package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"
	"github.com/redis/go-redis/v9"
)

// TaskStatus 任务状态
type TaskStatus struct {
	TaskID      string    `json:"task_id"`
	TaskType    string    `json:"task_type"`    // 'schema_parse', 'vector_index', 'sync'
	Status      string    `json:"status"`       // 'pending', 'running', 'success', 'failed'
	Progress    int       `json:"progress"`     // 0-100
	CurrentStep string    `json:"current_step"` // 当前步骤描述
	Result      string    `json:"result"`       // 成功时的结果（如schema_id）
	ErrorMsg    string    `json:"error_msg"`    // 失败时的错误信息
	StartedAt   time.Time `json:"started_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TaskCache Redis任务缓存管理器
type TaskCache struct {
	client *redis.Client
	prefix string
	ttl    time.Duration
}

// NewTaskCache 创建任务缓存管理器
func NewTaskCache(client *redis.Client) *TaskCache {
	return &TaskCache{
		client: client,
		prefix: "nl2sql:task:",
		ttl:    2 * time.Hour, // 任务2小时后自动过期
	}
}

// SaveTask 保存任务状态
func (tc *TaskCache) SaveTask(ctx context.Context, task *TaskStatus) error {
	task.UpdatedAt = time.Now()

	taskJSON, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("marshal task failed: %w", err)
	}

	key := tc.prefix + task.TaskID
	return tc.client.Set(ctx, key, taskJSON, tc.ttl).Err()
}

// GetTask 获取任务状态
func (tc *TaskCache) GetTask(ctx context.Context, taskID string) (*TaskStatus, error) {
	key := tc.prefix + taskID

	taskJSON, err := tc.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("任务不存在或已过期")
	}
	if err != nil {
		return nil, fmt.Errorf("get task from redis failed: %w", err)
	}

	var task TaskStatus
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return nil, fmt.Errorf("unmarshal task failed: %w", err)
	}

	return &task, nil
}

// UpdateTask 更新任务状态
func (tc *TaskCache) UpdateTask(ctx context.Context, taskID string, updateFn func(*TaskStatus)) error {
	task, err := tc.GetTask(ctx, taskID)
	if err != nil {
		return err
	}

	updateFn(task)
	return tc.SaveTask(ctx, task)
}

// DeleteTask 删除任务
func (tc *TaskCache) DeleteTask(ctx context.Context, taskID string) error {
	key := tc.prefix + taskID
	return tc.client.Del(ctx, key).Err()
}

// UpdateProgress 更新任务进度
func (tc *TaskCache) UpdateProgress(ctx context.Context, taskID string, progress int, currentStep string) error {
	return tc.UpdateTask(ctx, taskID, func(task *TaskStatus) {
		task.Status = nl2sqlCommon.TaskStatusRunning
		task.Progress = progress
		task.CurrentStep = currentStep
	})
}

// MarkSuccess 标记任务成功
func (tc *TaskCache) MarkSuccess(ctx context.Context, taskID string, result string) error {
	return tc.UpdateTask(ctx, taskID, func(task *TaskStatus) {
		task.Status = nl2sqlCommon.TaskStatusSuccess
		task.Progress = 100
		task.CurrentStep = "完成"
		task.Result = result
	})
}

// MarkFailed 标记任务失败
func (tc *TaskCache) MarkFailed(ctx context.Context, taskID string, errorMsg string) error {
	return tc.UpdateTask(ctx, taskID, func(task *TaskStatus) {
		task.Status = nl2sqlCommon.TaskStatusFailed
		task.ErrorMsg = errorMsg
	})
}
