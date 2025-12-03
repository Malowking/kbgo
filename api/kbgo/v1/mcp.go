package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// MCPRegistryCreateReq MCP service registration request
type MCPRegistryCreateReq struct {
	g.Meta      `path:"/v1/mcp/registry" method:"post" tags:"mcp" summary:"Register MCP service"`
	Name        string `v:"required|length:1,100" dc:"MCP service name (unique)"`
	Description string `v:"length:0,500" dc:"Service description"`
	Endpoint    string `v:"required|url" dc:"SSE endpoint URL"`
	ApiKey      string `v:"length:0,500" dc:"Authentication API key (optional)"`
	Headers     string `v:"json" dc:"Custom headers in JSON format (optional)"`
	Timeout     *int   `v:"min:1|max:300" dc:"Timeout in seconds (default: 30)"`
}

type MCPRegistryCreateRes struct {
	Id string `json:"id" dc:"MCP registry ID"`
}

// MCPRegistryUpdateReq MCP service update request
type MCPRegistryUpdateReq struct {
	g.Meta      `path:"/v1/mcp/registry/{id}" method:"put" tags:"mcp" summary:"Update MCP service"`
	Id          string  `v:"required" dc:"MCP registry ID"`
	Name        *string `v:"length:1,100" dc:"MCP service name"`
	Description *string `v:"length:0,500" dc:"Service description"`
	Endpoint    *string `v:"url" dc:"SSE endpoint URL"`
	ApiKey      *string `v:"length:0,500" dc:"Authentication API key"`
	Headers     *string `v:"json" dc:"Custom headers in JSON format"`
	Timeout     *int    `v:"min:1|max:300" dc:"Timeout in seconds"`
	Status      *int8   `v:"in:0,1" dc:"Status: 1-enabled, 0-disabled"`
}

type MCPRegistryUpdateRes struct{}

// MCPRegistryDeleteReq MCP service deletion request
type MCPRegistryDeleteReq struct {
	g.Meta `path:"/v1/mcp/registry/{id}" method:"delete" tags:"mcp" summary:"Delete MCP service"`
	Id     string `v:"required" dc:"MCP registry ID"`
}

type MCPRegistryDeleteRes struct{}

// MCPRegistryGetOneReq Get single MCP service request
type MCPRegistryGetOneReq struct {
	g.Meta `path:"/v1/mcp/registry/{id}" method:"get" tags:"mcp" summary:"Get one MCP service"`
	Id     string `v:"required" dc:"MCP registry ID"`
}

type MCPRegistryGetOneRes struct {
	Id          string `json:"id" dc:"MCP registry ID"`
	Name        string `json:"name" dc:"Service name"`
	Description string `json:"description" dc:"Service description"`
	Endpoint    string `json:"endpoint" dc:"SSE endpoint URL"`
	ApiKey      string `json:"api_key,omitempty" dc:"API key (masked)"`
	Headers     string `json:"headers,omitempty" dc:"Custom headers"`
	Timeout     int    `json:"timeout" dc:"Timeout in seconds"`
	Status      int8   `json:"status" dc:"Status: 1-enabled, 0-disabled"`
	CreateTime  string `json:"create_time" dc:"Create time"`
	UpdateTime  string `json:"update_time" dc:"Update time"`
}

// MCPRegistryGetListReq Get MCP services list request
type MCPRegistryGetListReq struct {
	g.Meta   `path:"/v1/mcp/registry" method:"get" tags:"mcp" summary:"Get MCP services list"`
	Status   *int8 `v:"in:0,1" dc:"Status filter: 1-enabled, 0-disabled"`
	Page     int   `v:"min:1" d:"1" dc:"Page number"`
	PageSize int   `v:"min:1|max:100" d:"10" dc:"Page size"`
}

type MCPRegistryGetListRes struct {
	List  []*MCPRegistryItem `json:"list" dc:"MCP services list"`
	Total int64              `json:"total" dc:"Total count"`
	Page  int                `json:"page" dc:"Current page"`
}

type MCPRegistryItem struct {
	Id          string `json:"id" dc:"MCP registry ID"`
	Name        string `json:"name" dc:"Service name"`
	Description string `json:"description" dc:"Service description"`
	Endpoint    string `json:"endpoint" dc:"SSE endpoint URL"`
	Timeout     int    `json:"timeout" dc:"Timeout in seconds"`
	Status      int8   `json:"status" dc:"Status: 1-enabled, 0-disabled"`
	CreateTime  string `json:"create_time" dc:"Create time"`
	UpdateTime  string `json:"update_time" dc:"Update time"`
}

// MCPRegistryUpdateStatusReq Update MCP service status request
type MCPRegistryUpdateStatusReq struct {
	g.Meta `path:"/v1/mcp/registry/{id}/status" method:"patch" tags:"mcp" summary:"Update MCP service status"`
	Id     string `v:"required" dc:"MCP registry ID"`
	Status int8   `v:"required|in:0,1" dc:"Status: 1-enabled, 0-disabled"`
}

type MCPRegistryUpdateStatusRes struct{}

// MCPRegistryTestReq Test MCP service connectivity request
type MCPRegistryTestReq struct {
	g.Meta `path:"/v1/mcp/registry/{id}/test" method:"post" tags:"mcp" summary:"Test MCP service connectivity"`
	Id     string `v:"required" dc:"MCP registry ID"`
}

type MCPRegistryTestRes struct {
	Success bool   `json:"success" dc:"Test result"`
	Message string `json:"message" dc:"Test message"`
}

// MCPListToolsReq List MCP service tools request
type MCPListToolsReq struct {
	g.Meta   `path:"/v1/mcp/registry/{id}/tools" method:"get" tags:"mcp" summary:"List MCP service tools"`
	Id       string `v:"required" dc:"MCP registry ID"`
	Cached   *bool  `d:"true" dc:"Use cached tools list"`
	CacheTTL *int   `d:"300" dc:"Cache TTL in seconds"`
}

type MCPListToolsRes struct {
	Tools []MCPToolInfo `json:"tools" dc:"Available tools"`
}

type MCPToolInfo struct {
	Name        string                 `json:"name" dc:"Tool name"`
	Description string                 `json:"description" dc:"Tool description"`
	InputSchema map[string]interface{} `json:"inputSchema" dc:"Input schema"`
}

// MCPCallToolReq Call MCP tool request
type MCPCallToolReq struct {
	g.Meta         `path:"/v1/mcp/call" method:"post" tags:"mcp" summary:"Call MCP tool"`
	RegistryID     string                 `v:"required" dc:"MCP registry ID or service name" json:"registry_id"`
	ToolName       string                 `v:"required" dc:"Tool name" json:"tool_name"`
	Arguments      map[string]interface{} `dc:"Tool arguments" json:"arguments"`
	ConversationID string                 `dc:"Conversation ID for logging" json:"conversation_id"`
}

type MCPCallToolRes struct {
	Content []MCPContentItem `json:"content" dc:"Result content"`
	IsError bool             `json:"is_error" dc:"Whether result is error"`
	LogID   string           `json:"log_id" dc:"Call log ID"`
}

type MCPContentItem struct {
	Type string `json:"type" dc:"Content type: text, image, resource"`
	Text string `json:"text,omitempty" dc:"Text content"`
	Data string `json:"data,omitempty" dc:"Binary data (base64)"`
}

// MCPCallLogGetListReq Get MCP call logs list request
type MCPCallLogGetListReq struct {
	g.Meta         `path:"/v1/mcp/logs" method:"get" tags:"mcp" summary:"Get MCP call logs"`
	ConversationID *string `dc:"Filter by conversation ID" json:"conversation_id"`
	RegistryID     *string `dc:"Filter by MCP registry ID" json:"registry_id"`
	ServiceName    *string `dc:"Filter by MCP service name" json:"service_name"`
	ToolName       *string `dc:"Filter by tool name" json:"tool_name"`
	Status         *int8   `v:"in:0,1,2" dc:"Status: 1-success, 0-failed, 2-timeout" json:"status"`
	StartTime      *string `dc:"Start time (RFC3339)" json:"start_time"`
	EndTime        *string `dc:"End time (RFC3339)" json:"end_time"`
	Page           int     `v:"min:1" d:"1" dc:"Page number" json:"page"`
	PageSize       int     `v:"min:1|max:100" d:"10" dc:"Page size" json:"page_size"`
}

type MCPCallLogGetListRes struct {
	List  []*MCPCallLogItem `json:"list" dc:"Call logs list"`
	Total int64             `json:"total" dc:"Total count"`
	Page  int               `json:"page" dc:"Current page"`
}

type MCPCallLogItem struct {
	Id              string `json:"id" dc:"Log ID"`
	ConversationID  string `json:"conversation_id" dc:"Conversation ID"`
	MCPRegistryID   string `json:"mcp_registry_id" dc:"MCP registry ID"`
	MCPServiceName  string `json:"mcp_service_name" dc:"MCP service name"`
	ToolName        string `json:"tool_name" dc:"Tool name"`
	RequestPayload  string `json:"request_payload" dc:"Request payload (JSON)"`
	ResponsePayload string `json:"response_payload" dc:"Response payload (JSON)"`
	Status          int8   `json:"status" dc:"Status: 1-success, 0-failed, 2-timeout"`
	ErrorMessage    string `json:"error_message,omitempty" dc:"Error message"`
	Duration        int    `json:"duration" dc:"Duration in milliseconds"`
	CreateTime      string `json:"create_time" dc:"Create time"`
}

// MCPCallLogGetByConversationReq Get call logs by conversation ID request
type MCPCallLogGetByConversationReq struct {
	g.Meta         `path:"/v1/mcp/logs/conversation/{conversation_id}" method:"get" tags:"mcp" summary:"Get MCP call logs by conversation ID"`
	ConversationID string `v:"required" dc:"Conversation ID"`
	Page           int    `v:"min:1" d:"1" dc:"Page number"`
	PageSize       int    `v:"min:1|max:100" d:"10" dc:"Page size"`
}

type MCPCallLogGetByConversationRes struct {
	List  []*MCPCallLogItem `json:"list" dc:"Call logs list"`
	Total int64             `json:"total" dc:"Total count"`
	Page  int               `json:"page" dc:"Current page"`
}

// MCPRegistryStatsReq Get MCP service statistics request
type MCPRegistryStatsReq struct {
	g.Meta `path:"/v1/mcp/registry/{id}/stats" method:"get" tags:"mcp" summary:"Get MCP service statistics"`
	Id     string `v:"required" dc:"MCP registry ID"`
}

type MCPRegistryStatsRes struct {
	TotalCalls   int64   `json:"total_calls" dc:"Total calls count"`
	SuccessCalls int64   `json:"success_calls" dc:"Success calls count"`
	FailedCalls  int64   `json:"failed_calls" dc:"Failed calls count"`
	AvgDuration  float32 `json:"avg_duration" dc:"Average duration in milliseconds"`
}
