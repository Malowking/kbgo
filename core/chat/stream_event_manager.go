package chat

import (
	"context"

	"github.com/Malowking/kbgo/core/common"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// StreamEventManager 流式事件管理器
type StreamEventManager struct {
	ctx      context.Context
	response *ghttp.Response
}

// NewStreamEventManager 创建流式事件管理器
func NewStreamEventManager(ctx context.Context) *StreamEventManager {
	r := g.RequestFromCtx(ctx)
	if r == nil {
		return nil
	}

	return &StreamEventManager{
		ctx:      ctx,
		response: r.Response,
	}
}

// SendToolPlanStart 发送工具计划开始事件
func (m *StreamEventManager) SendToolPlanStart(messageID string) error {
	common.WriteToolPlanStart(m.response, messageID)
	return nil
}

// SendToolPlanThinking 发送工具计划思考事件
func (m *StreamEventManager) SendToolPlanThinking(messageID, content string) error {
	common.WriteToolPlanThinking(m.response, messageID, content)
	return nil
}

// SendToolPlanComplete 发送工具计划完成事件
func (m *StreamEventManager) SendToolPlanComplete(messageID string, stepsCount int, needTools bool) error {
	common.WriteToolPlanComplete(m.response, messageID, stepsCount, needTools)
	return nil
}

// SendToolExecutionStart 发送工具执行开始事件
func (m *StreamEventManager) SendToolExecutionStart(messageID, stepID, toolName, reason string) error {
	common.WriteToolExecutionStart(m.response, messageID, stepID, toolName, reason)
	return nil
}

// SendToolExecutionComplete 发送工具执行完成事件
func (m *StreamEventManager) SendToolExecutionComplete(messageID, stepID, toolName, resultSummary string) error {
	common.WriteToolExecutionComplete(m.response, messageID, stepID, toolName, resultSummary)
	return nil
}

// SendToolExecutionError 发送工具执行错误事件
func (m *StreamEventManager) SendToolExecutionError(messageID, stepID, toolName, errorMsg string) error {
	common.WriteToolExecutionError(m.response, messageID, stepID, toolName, errorMsg)
	return nil
}

// SendFinalAnswerStart 发送最终答案开始事件
func (m *StreamEventManager) SendFinalAnswerStart(messageID string) error {
	common.WriteFinalAnswerStart(m.response, messageID)
	return nil
}
