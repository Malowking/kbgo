package async

// TODO: 异步任务功能暂时不需要，先注释掉

/*
import (
	"context"
	"fmt"
	"time"

	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/model/gorm"
	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"

	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/google/uuid"
	gormDB "gorm.io/gorm"
)

// TaskDAO 任务数据访问对象
type TaskDAO struct{}

// NewTaskDAO 创建TaskDAO
func NewTaskDAO() *TaskDAO {
	return &TaskDAO{}
}

// Create 创建任务
func (dao *TaskDAO) Create(ctx context.Context, task *Task) error {
	// 生成UUID
	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	// 序列化Config
	configJSON, err := gjson.Marshal(task.Config)
	if err != nil {
		return fmt.Errorf("序列化任务配置失败: %w", err)
	}

	record := gorm.NL2SQLAsyncTask{
		ID:       task.ID,
		AgentID:  task.AgentID,
		TaskType: task.TaskType,
		Status:   nl2sqlCommon.TaskStatusPending,
		Progress: 0,
		Result:   configJSON,
	}

	// 插入数据库
	err = dao.GetDB().WithContext(ctx).Create(&record).Error
	if err != nil {
		return fmt.Errorf("创建任务失败: %w", err)
	}

	return nil
}

// GetPendingTasks 获取待处理任务
func (dao *TaskDAO) GetPendingTasks(ctx context.Context, limit int) ([]*Task, error) {
	var records []gorm.NL2SQLAsyncTask
	err := g.DB(consts.DatabaseDefault).
		Model("nl2sql_async_tasks").
		Ctx(ctx).
		Where("status", nl2sqlCommon.TaskStatusPending).
		OrderAsc("created_at").
		Limit(limit).
		Scan(&records)

	if err != nil && err != gdb.ErrRecordNotFound {
		return nil, fmt.Errorf("查询待处理任务失败: %w", err)
	}

	tasks := make([]*Task, 0, len(records))
	for _, record := range records {
		task := &Task{
			ID:       record.ID,
			AgentID:  record.AgentID,
			TaskType: record.TaskType,
			Config:   make(map[string]interface{}),
		}

		// 反序列化Config
		if len(record.Result) > 0 {
			if err := gjson.Unmarshal(record.Result, &task.Config); err != nil {
				continue // 跳过无效配置的任务
			}
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

// UpdateStatus 更新任务状态
func (dao *TaskDAO) UpdateStatus(ctx context.Context, taskID string, status string) error {
	_, err := g.DB(consts.DatabaseDefault).
		Model("nl2sql_async_tasks").
		Ctx(ctx).
		Data(g.Map{"status": status}).
		Where("id", taskID).
		Update()

	if err != nil {
		return fmt.Errorf("更新任务状态失败: %w", err)
	}

	return nil
}

// UpdateProgress 更新任务进度
func (dao *TaskDAO) UpdateProgress(ctx context.Context, taskID string, progress int, step string) error {
	_, err := g.DB(consts.DatabaseDefault).
		Model("nl2sql_async_tasks").
		Ctx(ctx).
		Data(g.Map{
			"progress":     progress,
			"current_step": step,
		}).
		Where("id", taskID).
		Update()

	if err != nil {
		return fmt.Errorf("更新任务进度失败: %w", err)
	}

	return nil
}

// UpdateStartedAt 更新任务开始时间
func (dao *TaskDAO) UpdateStartedAt(ctx context.Context, taskID string, startedAt *time.Time) error {
	_, err := g.DB(consts.DatabaseDefault).
		Model("nl2sql_async_tasks").
		Ctx(ctx).
		Data(g.Map{"started_at": startedAt}).
		Where("id", taskID).
		Update()

	return err
}

// MarkFailed 标记任务失败
func (dao *TaskDAO) MarkFailed(ctx context.Context, taskID string, errorMsg string, completedAt *time.Time) error {
	_, err := g.DB(consts.DatabaseDefault).
		Model("nl2sql_async_tasks").
		Ctx(ctx).
		Data(g.Map{
			"status":       nl2sqlCommon.TaskStatusFailed,
			"error_msg":    errorMsg,
			"completed_at": completedAt,
		}).
		Where("id", taskID).
		Update()

	return err
}

// MarkSuccess 标记任务成功
func (dao *TaskDAO) MarkSuccess(ctx context.Context, taskID string, completedAt *time.Time) error {
	_, err := g.DB(consts.DatabaseDefault).
		Model("nl2sql_async_tasks").
		Ctx(ctx).
		Data(g.Map{
			"status":       nl2sqlCommon.TaskStatusSuccess,
			"progress":     100,
			"completed_at": completedAt,
		}).
		Where("id", taskID).
		Update()

	return err
}

// GetStatus 获取任务状态
func (dao *TaskDAO) GetStatus(ctx context.Context, taskID string) (*TaskStatus, error) {
	var record gorm.NL2SQLAsyncTask
	err := g.DB(consts.DatabaseDefault).
		Model("nl2sql_async_tasks").
		Ctx(ctx).
		Where("id", taskID).
		Scan(&record)

	if err != nil {
		if err == gdb.ErrRecordNotFound {
			return nil, fmt.Errorf("任务不存在: %s", taskID)
		}
		return nil, fmt.Errorf("查询任务状态失败: %w", err)
	}

	status := &TaskStatus{
		ID:          record.ID,
		Status:      record.Status,
		Progress:    record.Progress,
		CurrentStep: record.CurrentStep,
		ErrorMsg:    record.ErrorMsg,
		StartedAt:   record.StartedAt,
		CompletedAt: record.CompletedAt,
	}

	return status, nil
}
*/
