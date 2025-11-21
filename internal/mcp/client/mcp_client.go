package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// MCPClient MCP 客户端
type MCPClient struct {
	registry      *gormModel.MCPRegistry
	httpClient    *http.Client
	sessionID     string // MCP session ID
	transportMode string // "sse" or "http"

	// SSE 模式相关
	sseConn         *http.Response // SSE 连接
	messageEndpoint string         // 消息发送端点
	sseReader       *bufio.Scanner
	responses       map[interface{}]chan *MCPResponse // 响应通道
	responsesMutex  sync.RWMutex
	connClosed      chan struct{}
	connMutex       sync.Mutex
}

// NewMCPClient 创建MCP客户端
func NewMCPClient(registry *gormModel.MCPRegistry) *MCPClient {
	timeout := time.Duration(registry.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	// 自动检测传输模式
	transportMode := "http" // 默认
	if strings.Contains(registry.Endpoint, "/sse") {
		transportMode = "sse"
	}

	return &MCPClient{
		registry:      registry,
		transportMode: transportMode,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		responses:  make(map[interface{}]chan *MCPResponse),
		connClosed: make(chan struct{}),
	}
}

// MCPRequest MCP请求结构
type MCPRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse MCP响应结构
type MCPResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError MCP错误结构
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    string `json:"data,omitempty"`
}

// MCPTool MCP工具定义
type MCPTool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// MCPToolsListResult 工具列表响应结果
type MCPToolsListResult struct {
	Tools []MCPTool `json:"tools"`
}

// MCPCallToolParams 调用工具参数
type MCPCallToolParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// MCPCallToolResult 调用工具结果
type MCPCallToolResult struct {
	Content []MCPContent `json:"content"`
	IsError bool         `json:"isError,omitempty"`
}

// MCPContent MCP内容结构
type MCPContent struct {
	Type string `json:"type"` // text, image, resource
	Text string `json:"text,omitempty"`
	Data string `json:"data,omitempty"`
	// 可扩展其他字段
}

// LLMTool LLM Function Calling 工具结构（兼容 OpenAI 格式）
type LLMTool struct {
	Type     string                `json:"type"` // "function"
	Function LLMFunctionDefinition `json:"function"`
}

// LLMFunctionDefinition LLM 函数定义
type LLMFunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema
}

// LLMToolCall LLM 工具调用结构
type LLMToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"` // "function"
	Function LLMFunctionCall `json:"function"`
}

// LLMFunctionCall LLM 函数调用
type LLMFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ListTools 列出所有可用工具
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPTool, error) {
	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      "tools-list", // 使用字符串ID确保类型匹配
		Method:  "tools/list", // 标准 MCP 方法名
		Params:  map[string]interface{}{},
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s - %s", resp.Error.Code, resp.Error.Message, resp.Error.Data)
	}

	var result MCPToolsListResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tools list result: %v", err)
	}

	return result.Tools, nil
}

// CallTool 调用指定工具
func (c *MCPClient) CallTool(ctx context.Context, toolName string, arguments map[string]interface{}) (*MCPCallToolResult, error) {
	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      fmt.Sprintf("call-%d", time.Now().UnixNano()),
		Method:  "tools/call",
		Params: MCPCallToolParams{
			Name:      toolName,
			Arguments: arguments,
		},
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s - %s", resp.Error.Code, resp.Error.Message, resp.Error.Data)
	}

	var result MCPCallToolResult
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse tool call result: %v", err)
	}

	return &result, nil
}

// sendRequest 发送MCP请求（支持 HTTP 和 SSE 模式）
func (c *MCPClient) sendRequest(ctx context.Context, mcpReq MCPRequest) (*MCPResponse, error) {
	if c.transportMode == "sse" {
		return c.sendSSERequest(ctx, mcpReq)
	}
	return c.sendHTTPRequest(ctx, mcpReq)
}

// sendHTTPRequest 发送HTTP模式的MCP请求
func (c *MCPClient) sendHTTPRequest(ctx context.Context, mcpReq MCPRequest) (*MCPResponse, error) {
	// 序列化请求
	reqBody, err := json.Marshal(mcpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// 创建HTTP请求
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.registry.Endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// 设置请求头
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/event-stream")

	// 如果有 session ID，则添加到请求头
	if c.sessionID != "" {
		httpReq.Header.Set("mcp-session-id", c.sessionID)
	}

	// 设置认证
	if c.registry.ApiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.registry.ApiKey)
	}

	// 设置自定义请求头
	if c.registry.Headers != "" {
		var customHeaders map[string]string
		if err := json.Unmarshal([]byte(c.registry.Headers), &customHeaders); err == nil {
			for k, v := range customHeaders {
				httpReq.Header.Set(k, v)
			}
		}
	}

	// 发送请求
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// 保存 session ID（如果有）
	if sessionID := resp.Header.Get("mcp-session-id"); sessionID != "" {
		c.sessionID = sessionID
		g.Log().Debugf(ctx, "Received MCP session ID: %s", sessionID)
	}

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// 读取SSE响应
	return c.readSSEResponse(resp.Body)
}

// sendSSERequest 发送SSE模式的MCP请求
func (c *MCPClient) sendSSERequest(ctx context.Context, mcpReq MCPRequest) (*MCPResponse, error) {
	// 确保 SSE 连接已建立
	if err := c.ensureSSEConnection(ctx); err != nil {
		return nil, fmt.Errorf("failed to establish SSE connection: %v", err)
	}

	// 创建响应通道
	respChan := make(chan *MCPResponse, 1)
	c.responsesMutex.Lock()
	c.responses[mcpReq.ID] = respChan
	c.responsesMutex.Unlock()

	// 确保在函数结束时清理响应通道
	defer func() {
		c.responsesMutex.Lock()
		delete(c.responses, mcpReq.ID)
		c.responsesMutex.Unlock()
		close(respChan)
	}()

	// 序列化请求
	reqBody, err := json.Marshal(mcpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %v", err)
	}

	// 发送消息到消息端点
	baseURL := strings.TrimSuffix(c.registry.Endpoint, "/sse")
	messageURL := fmt.Sprintf("%s%s", baseURL, c.messageEndpoint)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", messageURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create message request: %v", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// 设置认证
	if c.registry.ApiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.registry.ApiKey)
	}

	// 发送消息
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("failed to send message: %v", err)
	}
	defer resp.Body.Close()

	// 检查状态码（SSE模式应该返回202 Accepted）
	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to send message, status %d: %s", resp.StatusCode, string(body))
	}

	g.Log().Debugf(ctx, "Message sent successfully to SSE endpoint")

	// 等待响应
	select {
	case response := <-respChan:
		if response != nil {
			return response, nil
		}
		return nil, fmt.Errorf("received nil response")
	case <-ctx.Done():
		return nil, fmt.Errorf("request timeout: %v", ctx.Err())
	case <-time.After(30 * time.Second): // 额外的超时保护
		return nil, fmt.Errorf("SSE response timeout")
	}
}

// ensureSSEConnection 确保SSE连接已建立
func (c *MCPClient) ensureSSEConnection(ctx context.Context) error {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	// 如果连接已经建立，直接返回
	if c.sseConn != nil && c.messageEndpoint != "" {
		return nil
	}

	g.Log().Debugf(ctx, "Establishing SSE connection to %s", c.registry.Endpoint)

	// 建立 SSE 连接
	req, err := http.NewRequestWithContext(ctx, "GET", c.registry.Endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create SSE request: %v", err)
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	// 设置认证
	if c.registry.ApiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.registry.ApiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to SSE endpoint: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return fmt.Errorf("SSE connection failed with status %d", resp.StatusCode)
	}

	c.sseConn = resp
	c.sseReader = bufio.NewScanner(resp.Body)

	// 读取第一个事件以获取消息端点
	if err := c.readSSEEndpoint(ctx); err != nil {
		resp.Body.Close()
		c.sseConn = nil
		c.sseReader = nil
		return fmt.Errorf("failed to read SSE endpoint: %v", err)
	}

	// 启动 SSE 响应处理协程
	go c.handleSSEResponses(ctx)

	g.Log().Debugf(ctx, "SSE connection established, message endpoint: %s", c.messageEndpoint)
	return nil
}

// readSSEEndpoint 读取 SSE 端点信息
func (c *MCPClient) readSSEEndpoint(ctx context.Context) error {
	for c.sseReader.Scan() {
		line := c.sseReader.Text()

		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// 查找消息端点
			if strings.Contains(data, "/messages/") {
				re := regexp.MustCompile(`/messages/\?session_id=([a-f0-9]+)`)
				match := re.FindStringSubmatch(data)
				if len(match) > 1 {
					c.sessionID = match[1]
					c.messageEndpoint = data
					return nil
				}
			}
		}
	}

	return fmt.Errorf("failed to find message endpoint in SSE stream")
}

// handleSSEResponses 处理 SSE 响应
func (c *MCPClient) handleSSEResponses(ctx context.Context) {
	defer func() {
		if c.sseConn != nil {
			c.sseConn.Body.Close()
			c.sseConn = nil
		}
		close(c.connClosed)
		g.Log().Debugf(ctx, "SSE response handler stopped")
	}()

	var messageData []byte

	for c.sseReader.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := c.sseReader.Text()

		// 空行表示一条消息结束
		if line == "" {
			if len(messageData) > 0 {
				c.processSSEMessage(ctx, messageData)
				messageData = nil
			}
			continue
		}

		// 解析 SSE 字段
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			messageData = append(messageData, []byte(data)...)
		} else if strings.HasPrefix(line, "event: ") {
			event := strings.TrimPrefix(line, "event: ")
			g.Log().Debugf(ctx, "SSE event: %s", event)
		}
	}

	if err := c.sseReader.Err(); err != nil {
		g.Log().Errorf(ctx, "SSE reader error: %v", err)
	}
}

// processSSEMessage 处理 SSE 消息
func (c *MCPClient) processSSEMessage(ctx context.Context, data []byte) {
	var mcpResp MCPResponse
	if err := json.Unmarshal(data, &mcpResp); err != nil {
		g.Log().Warningf(ctx, "Failed to parse SSE message: %v, data: %s", err, string(data))
		return
	}

	g.Log().Debugf(ctx, "Received SSE response for ID: %v", mcpResp.ID)

	// 找到对应的响应通道
	c.responsesMutex.RLock()
	respChan, exists := c.responses[mcpResp.ID]
	c.responsesMutex.RUnlock()

	if exists && respChan != nil {
		select {
		case respChan <- &mcpResp:
			// 响应已发送
		default:
			// 通道已满或已关闭
			g.Log().Warningf(ctx, "Response channel full or closed for ID: %v", mcpResp.ID)
		}
	} else {
		g.Log().Warningf(ctx, "No response channel found for ID: %v", mcpResp.ID)
	}
}

// Close 关闭MCP客户端连接
func (c *MCPClient) Close() error {
	if c.transportMode == "sse" && c.sseConn != nil {
		c.sseConn.Body.Close()
		<-c.connClosed // 等待连接关闭
	}
	return nil
}

// readSSEResponse 读取SSE格式的响应
func (c *MCPClient) readSSEResponse(reader io.Reader) (*MCPResponse, error) {
	scanner := bufio.NewScanner(reader)
	var messageData []byte

	for scanner.Scan() {
		line := scanner.Text()

		// 空行表示一条消息结束
		if line == "" {
			if len(messageData) > 0 {
				var mcpResp MCPResponse
				if err := json.Unmarshal(messageData, &mcpResp); err != nil {
					g.Log().Warningf(context.Background(), "Failed to parse SSE message: %v, data: %s", err, string(messageData))
					messageData = nil
					continue
				}
				return &mcpResp, nil
			}
			continue
		}

		// 解析SSE字段
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			messageData = append(messageData, []byte(data)...)
		} else if strings.HasPrefix(line, "event: ") {
			// 可以处理不同类型的事件
			event := strings.TrimPrefix(line, "event: ")
			g.Log().Debugf(context.Background(), "SSE event: %s", event)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading SSE stream: %v", err)
	}

	return nil, fmt.Errorf("no valid SSE message received")
}

// Ping 测试MCP服务连通性
func (c *MCPClient) Ping(ctx context.Context) error {
	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      "ping",
		Method:  "ping",
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("ping failed: %s", resp.Error.Message)
	}

	return nil
}

// Initialize 初始化MCP连接
func (c *MCPClient) Initialize(ctx context.Context, clientInfo map[string]interface{}) error {
	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      "init",
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]interface{}{
				"tools": map[string]interface{}{},
			},
			"clientInfo": clientInfo,
		},
	}

	resp, err := c.sendRequest(ctx, req)
	if err != nil {
		return err
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize failed: %s", resp.Error.Message)
	}

	return nil
}

// ConvertToLLMTools 将 MCP 工具列表转换为 LLM Function Calling 格式
func ConvertToLLMTools(mcpTools []MCPTool, serviceName string) []LLMTool {
	llmTools := make([]LLMTool, 0, len(mcpTools))

	for _, mcpTool := range mcpTools {
		// 为工具名添加服务前缀，避免不同服务的工具名冲突
		toolName := fmt.Sprintf("%s__%s", serviceName, mcpTool.Name)

		// 构建 LLM 工具定义
		llmTool := LLMTool{
			Type: "function",
			Function: LLMFunctionDefinition{
				Name:        toolName,
				Description: mcpTool.Description,
				Parameters:  mcpTool.InputSchema,
			},
		}

		// 确保 parameters 有基本结构
		if llmTool.Function.Parameters == nil {
			llmTool.Function.Parameters = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		// 如果 InputSchema 没有 type 字段，添加默认的
		if _, hasType := llmTool.Function.Parameters["type"]; !hasType {
			llmTool.Function.Parameters["type"] = "object"
		}

		llmTools = append(llmTools, llmTool)
	}

	return llmTools
}

// ParseToolName 解析带服务前缀的工具名，返回 (serviceName, toolName)
func ParseToolName(fullToolName string) (string, string) {
	parts := strings.SplitN(fullToolName, "__", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// 如果没有前缀，返回空服务名和原工具名
	return "", fullToolName
}
