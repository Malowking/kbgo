package async

// TODO: 异步任务功能暂时不需要，先注释掉

/*
import (
	"context"
	"fmt"
	"sync"
	"time"

	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"
	"github.com/gogf/gf/v2/os/glog"
)

// TaskScheduler 异步任务调度器
type TaskScheduler struct {
	taskQueue   chan *Task
	workerCount int
	workers     []*Worker
	taskDAO     *TaskDAO
	mu          sync.RWMutex
	running     bool
}

// Task 任务定义
type Task struct {
	ID         string
	AgentID    string
	TaskType   string                 // 'schema_parse', 'vector_index', 'sync'
	Config     map[string]interface{} // 任务配置
	RetryCount int
}

// NewTaskScheduler 创建任务调度器
func NewTaskScheduler(workerCount int) *TaskScheduler {
	return &TaskScheduler{
		taskQueue:   make(chan *Task, 100),
		workerCount: workerCount,
		workers:     make([]*Worker, 0, workerCount),
		taskDAO:     NewTaskDAO(),
	}
}

// Start 启动调度器
func (s *TaskScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return fmt.Errorf("调度器已在运行")
	}
	s.running = true
	s.mu.Unlock()

	// 启动 Worker
	for i := 0; i < s.workerCount; i++ {
		worker := NewWorker(i, s.taskQueue, s.taskDAO)
		s.workers = append(s.workers, worker)
		go worker.Start(ctx)
	}

	// 启动任务轮询
	go s.pollTasks(ctx)

	glog.Infof(ctx, "任务调度器启动成功，Worker数量: %d", s.workerCount)
	return nil
}

// Stop 停止调度器
func (s *TaskScheduler) Stop(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	s.running = false
	close(s.taskQueue)

	glog.Info(ctx, "任务调度器已停止")
}

// SubmitTask 提交任务
func (s *TaskScheduler) SubmitTask(ctx context.Context, task *Task) error {
	// 1. 保存到数据库
	if err := s.taskDAO.Create(ctx, task); err != nil {
		return fmt.Errorf("保存任务失败: %w", err)
	}

	// 2. 加入队列
	select {
	case s.taskQueue <- task:
		glog.Infof(ctx, "任务已提交: ID=%s, Type=%s", task.ID, task.TaskType)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return fmt.Errorf("任务队列已满")
	}
}

// pollTasks 定期从数据库加载待处理任务
func (s *TaskScheduler) pollTasks(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.loadPendingTasks(ctx)
		}
	}
}

// loadPendingTasks 加载待处理任务
func (s *TaskScheduler) loadPendingTasks(ctx context.Context) {
	tasks, err := s.taskDAO.GetPendingTasks(ctx, 10)
	if err != nil {
		glog.Errorf(ctx, "加载待处理任务失败: %v", err)
		return
	}

	for _, task := range tasks {
		// 更新状态为 running
		if err := s.taskDAO.UpdateStatus(ctx, task.ID, nl2sqlCommon.TaskStatusRunning); err != nil {
			glog.Errorf(ctx, "更新任务状态失败: %v", err)
			continue
		}

		// 提交到队列
		select {
		case s.taskQueue <- task:
			glog.Debugf(ctx, "任务已加入队列: %s", task.ID)
		case <-ctx.Done():
			return
		default:
			glog.Warningf(ctx, "任务队列已满，跳过任务: %s", task.ID)
			// 恢复为 pending 状态
			s.taskDAO.UpdateStatus(ctx, task.ID, nl2sqlCommon.TaskStatusPending)
		}
	}
}

// GetTaskStatus 获取任务状态
func (s *TaskScheduler) GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error) {
	return s.taskDAO.GetStatus(ctx, taskID)
}

// TaskStatus 任务状态
type TaskStatus struct {
	ID          string
	Status      string
	Progress    int
	CurrentStep string
	ErrorMsg    string
	StartedAt   *time.Time
	CompletedAt *time.Time
}
*/
