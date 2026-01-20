package schema

import "time"

// StreamEventType 流式事件类型
type StreamEventType string

const (
	// 工具计划生成阶段
	EventToolPlanStart    StreamEventType = "tool_plan_start"    // 开始生成工具执行计划
	EventToolPlanThinking StreamEventType = "tool_plan_thinking" // LLM 思考过程（流式输出）
	EventToolPlanComplete StreamEventType = "tool_plan_complete" // 工具执行计划生成完成

	// 工具执行阶段
	EventToolExecutionStart    StreamEventType = "tool_execution_start"    // 开始执行某个工具
	EventToolExecutionProgress StreamEventType = "tool_execution_progress" // 工具执行进度
	EventToolExecutionComplete StreamEventType = "tool_execution_complete" // 工具执行完成
	EventToolExecutionError    StreamEventType = "tool_execution_error"    // 工具执行失败

	// 最终答案生成阶段
	EventFinalAnswerStart    StreamEventType = "final_answer_start"    // 开始生成最终答案
	EventFinalAnswerThinking StreamEventType = "final_answer_thinking" // LLM 思考过程（流式输出）
	EventFinalAnswerComplete StreamEventType = "final_answer_complete" // 最终答案生成完成
)

// ToolExecutionPlan 工具执行计划
type ToolExecutionPlan struct {
	MessageID string               `json:"message_id"` // 消息ID
	NeedTools bool                 `json:"need_tools"` // 是否需要使用工具
	Steps     []*ToolExecutionStep `json:"steps"`      // 执行步骤列表
	Reasoning string               `json:"reasoning"`  // LLM的推理过程
	CreatedAt time.Time            `json:"created_at"` // 创建时间
}

// ToolExecutionStep 工具执行步骤
type ToolExecutionStep struct {
	StepID     string                 `json:"step_id"`    // 步骤ID
	ToolName   string                 `json:"tool_name"`  // 工具名称
	ToolType   string                 `json:"tool_type"`  // 工具类型
	Parameters map[string]interface{} `json:"parameters"` // 执行参数
	Reason     string                 `json:"reason"`     // 执行原因
	DependsOn  []string               `json:"depends_on"` // 依赖的步骤ID列表
	Status     StepStatus             `json:"status"`     // 执行状态
	Result     *ToolResult            `json:"result"`     // 执行结果
	Error      string                 `json:"error"`      // 错误信息
	StartTime  *time.Time             `json:"start_time"` // 开始时间
	EndTime    *time.Time             `json:"end_time"`   // 结束时间
}

// StepStatus 步骤执行状态
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"   // 等待执行
	StepStatusRunning   StepStatus = "running"   // 执行中
	StepStatusCompleted StepStatus = "completed" // 执行完成
	StepStatusFailed    StepStatus = "failed"    // 执行失败
	StepStatusSkipped   StepStatus = "skipped"   // 跳过执行
)

// ToolExecutionEvent 工具执行事件
type ToolExecutionEvent struct {
	MessageID string                 `json:"message_id"` // 消息ID
	EventType StreamEventType        `json:"event_type"` // 事件类型
	StepID    string                 `json:"step_id"`    // 步骤ID（可选）
	Content   string                 `json:"content"`    // 事件内容
	Data      map[string]interface{} `json:"data"`       // 附加数据
	Timestamp time.Time              `json:"timestamp"`  // 时间戳
}

// NewToolExecutionPlan 创建工具执行计划
func NewToolExecutionPlan(messageID string) *ToolExecutionPlan {
	return &ToolExecutionPlan{
		MessageID: messageID,
		Steps:     make([]*ToolExecutionStep, 0),
		CreatedAt: time.Now(),
	}
}

// AddStep 添加执行步骤
func (p *ToolExecutionPlan) AddStep(step *ToolExecutionStep) {
	p.Steps = append(p.Steps, step)
}

// GetStep 根据步骤ID获取步骤
func (p *ToolExecutionPlan) GetStep(stepID string) *ToolExecutionStep {
	for _, step := range p.Steps {
		if step.StepID == stepID {
			return step
		}
	}
	return nil
}

// NewToolExecutionStep 创建工具执行步骤
func NewToolExecutionStep(stepID, toolName, toolType string) *ToolExecutionStep {
	return &ToolExecutionStep{
		StepID:     stepID,
		ToolName:   toolName,
		ToolType:   toolType,
		Parameters: make(map[string]interface{}),
		DependsOn:  make([]string, 0),
		Status:     StepStatusPending,
	}
}

// MarkAsRunning 标记为执行中
func (s *ToolExecutionStep) MarkAsRunning() {
	s.Status = StepStatusRunning
	now := time.Now()
	s.StartTime = &now
}

// MarkAsCompleted 标记为执行完成
func (s *ToolExecutionStep) MarkAsCompleted(result *ToolResult) {
	s.Status = StepStatusCompleted
	s.Result = result
	now := time.Now()
	s.EndTime = &now
}

// MarkAsFailed 标记为执行失败
func (s *ToolExecutionStep) MarkAsFailed(err error) {
	s.Status = StepStatusFailed
	s.Error = err.Error()
	now := time.Now()
	s.EndTime = &now
}

// MarkAsSkipped 标记为跳过
func (s *ToolExecutionStep) MarkAsSkipped(reason string) {
	s.Status = StepStatusSkipped
	s.Error = reason
}

// NewToolExecutionEvent 创建工具执行事件
func NewToolExecutionEvent(messageID string, eventType StreamEventType, content string) *ToolExecutionEvent {
	return &ToolExecutionEvent{
		MessageID: messageID,
		EventType: eventType,
		Content:   content,
		Data:      make(map[string]interface{}),
		Timestamp: time.Now(),
	}
}

// WithStepID 设置步骤ID
func (e *ToolExecutionEvent) WithStepID(stepID string) *ToolExecutionEvent {
	e.StepID = stepID
	return e
}

// WithData 添加附加数据
func (e *ToolExecutionEvent) WithData(key string, value interface{}) *ToolExecutionEvent {
	e.Data[key] = value
	return e
}
