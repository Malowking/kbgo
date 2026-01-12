package common

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/gogf/gf/v2/os/gctx"

	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/bytedance/sonic"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"github.com/google/uuid"
)

// GenerateMessageID 生成消息ID
func GenerateMessageID() string {
	return uuid.NewString()
}

type StreamData struct {
	Id               string                 `json:"id"`                          // 同一个消息里面的id是相同的
	Created          int64                  `json:"created"`                     // 消息初始生成时间
	Type             string                 `json:"type,omitempty"`              // 事件类型: content, tool_call_start, tool_call_end, llm_iteration, thinking
	Content          string                 `json:"content"`                     // 消息具体内容
	ReasoningContent string                 `json:"reasoning_content,omitempty"` // 思考内容（用于思考模型）
	Document         []*schema.Document     `json:"document"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"` // 元数据（用于工具调用、LLM迭代等信息）
}

func SteamResponse(ctx context.Context, streamReader schema.StreamReaderInterface[*schema.Message], docs []*schema.Document) (err error) {
	// 获取HTTP响应对象
	httpReq := ghttp.RequestFromCtx(ctx)
	httpResp := httpReq.Response
	// 设置响应头
	httpResp.Header().Set("Content-Type", "text/event-stream")
	httpResp.Header().Set("Cache-Control", "no-cache")
	httpResp.Header().Set("Connection", "keep-alive")
	httpResp.Header().Set("X-Accel-Buffering", "no") // 禁用Nginx缓冲
	httpResp.Header().Set("Access-Control-Allow-Origin", "*")
	sd := &StreamData{
		Id:      uuid.NewString(),
		Created: time.Now().Unix(),
	}

	// 处理流式响应
	for {
		chunk, err := streamReader.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			writeSSEError(httpResp, err)
			break
		}
		// 如果内容和思考内容都为空，跳过
		if len(chunk.Content) == 0 && len(chunk.ReasoningContent) == 0 {
			continue
		}

		sd.Content = chunk.Content
		sd.ReasoningContent = chunk.ReasoningContent
		marshal, _ := sonic.Marshal(sd)
		// 发送数据事件
		writeSSEData(httpResp, string(marshal))
	}

	// 在流式响应结束后，发送知识库检索结果作为最后一条消息
	if len(docs) > 0 {
		sd.Document = docs
		sd.Content = ""
		sd.ReasoningContent = ""
		marshal, _ := sonic.Marshal(sd)
		writeSSEDocuments(httpResp, string(marshal))
	}

	// 发送结束事件
	writeSSEDone(httpResp)
	return nil
}

// writeSSEData 写入SSE事件
func writeSSEData(resp *ghttp.Response, data string) {
	if len(data) == 0 {
		return
	}
	// g.Log().Infof(gctx.New(), "data: %s", data)
	resp.Writeln(fmt.Sprintf("data:%s\n", data))
	resp.Flush()
}

func writeSSEDone(resp *ghttp.Response) {
	resp.Writeln(fmt.Sprintf("data:%s\n", "[DONE]"))
	resp.Flush()
}

func writeSSEDocuments(resp *ghttp.Response, data string) {
	resp.Writeln(fmt.Sprintf("documents:%s\n", data))
	resp.Flush()
}

// writeSSEError 写入SSE错误
func writeSSEError(resp *ghttp.Response, err error) {
	g.Log().Error(gctx.New(), err)
	resp.Writeln(fmt.Sprintf("event: error\ndata: %s\n\n", err.Error()))
	resp.Flush()
}

// WriteToolCallStart 发送工具调用开始事件
func WriteToolCallStart(resp *ghttp.Response, messageID string, toolID string, toolName string, arguments map[string]interface{}) {
	sd := &StreamData{
		Id:      messageID,
		Created: time.Now().Unix(),
		Type:    "tool_call_start",
		Metadata: map[string]interface{}{
			"tool_id":   toolID,
			"tool_name": toolName,
			"arguments": arguments,
		},
	}
	marshal, _ := sonic.Marshal(sd)
	writeSSEData(resp, string(marshal))
}

// WriteToolCallEnd 发送工具调用结束事件
func WriteToolCallEnd(resp *ghttp.Response, messageID string, toolID string, toolName string, result string, err error, durationMs int64, fileURL ...string) {
	metadata := map[string]interface{}{
		"tool_id":     toolID,
		"tool_name":   toolName,
		"result":      result,
		"duration_ms": durationMs,
	}
	if err != nil {
		metadata["error"] = err.Error()
	}
	// 如果提供了 fileURL，添加到 metadata
	if len(fileURL) > 0 && fileURL[0] != "" {
		metadata["file_url"] = fileURL[0]
	}

	sd := &StreamData{
		Id:       messageID,
		Created:  time.Now().Unix(),
		Type:     "tool_call_end",
		Metadata: metadata,
	}
	marshal, _ := sonic.Marshal(sd)
	writeSSEData(resp, string(marshal))
}

// WriteLLMIteration 发送LLM迭代事件
func WriteLLMIteration(resp *ghttp.Response, messageID string, iteration int, maxIterations int, message string) {
	sd := &StreamData{
		Id:      messageID,
		Created: time.Now().Unix(),
		Type:    "llm_iteration",
		Metadata: map[string]interface{}{
			"iteration":      iteration,
			"max_iterations": maxIterations,
			"message":        message,
		},
	}
	marshal, _ := sonic.Marshal(sd)
	writeSSEData(resp, string(marshal))
}

// WriteThinking 发送思考过程事件
func WriteThinking(resp *ghttp.Response, messageID string, thinking string) {
	sd := &StreamData{
		Id:      messageID,
		Created: time.Now().Unix(),
		Type:    "thinking",
		Content: thinking,
	}
	marshal, _ := sonic.Marshal(sd)
	writeSSEData(resp, string(marshal))
}

// WriteSkillProgress 发送 Skill 执行进度事件
func WriteSkillProgress(resp *ghttp.Response, messageID string, toolID string, stage string, message string, metadata map[string]interface{}) {
	if resp == nil {
		return
	}

	meta := map[string]interface{}{
		"tool_id": toolID,
		"stage":   stage,
		"message": message,
	}

	// 合并额外的 metadata
	for k, v := range metadata {
		meta[k] = v
	}

	sd := &StreamData{
		Id:       messageID,
		Created:  time.Now().Unix(),
		Type:     "skill_progress",
		Metadata: meta,
	}
	marshal, _ := sonic.Marshal(sd)
	writeSSEData(resp, string(marshal))
}
