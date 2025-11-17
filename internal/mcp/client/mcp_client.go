package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// MCPClient MCP SSE客户端
type MCPClient struct {
	registry   *gormModel.MCPRegistry
	httpClient *http.Client
}

// NewMCPClient 创建MCP客户端
func NewMCPClient(registry *gormModel.MCPRegistry) *MCPClient {
	timeout := time.Duration(registry.Timeout) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &MCPClient{
		registry: registry,
		httpClient: &http.Client{
			Timeout: timeout,
		},
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

// ListTools 列出所有可用工具
func (c *MCPClient) ListTools(ctx context.Context) ([]MCPTool, error) {
	req := MCPRequest{
		Jsonrpc: "2.0",
		ID:      1,
		Method:  "tools/list",
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
		ID:      time.Now().UnixNano(),
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

// sendRequest 发送MCP请求（使用SSE）
func (c *MCPClient) sendRequest(ctx context.Context, mcpReq MCPRequest) (*MCPResponse, error) {
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
	httpReq.Header.Set("Accept", "text/event-stream")

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

	// 检查HTTP状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP error %d: %s", resp.StatusCode, string(body))
	}

	// 读取SSE响应
	return c.readSSEResponse(resp.Body)
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
