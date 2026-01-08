package v1

import (
	"github.com/gogf/gf/v2/frame/g"
)

// SkillCreateReq 创建 Skill 请求
type SkillCreateReq struct {
	g.Meta          `path:"/v1/skills" method:"post" tags:"skills" summary:"Create a new skill"`
	Name            string                           `v:"required|length:1,100" dc:"Skill name"`
	Description     string                           `v:"length:0,500" dc:"Skill description"`
	Version         string                           `v:"length:0,50" dc:"Version (e.g., 1.0.0)"`
	Author          string                           `v:"length:0,100" dc:"Author name"`
	Category        string                           `v:"length:0,50" dc:"Category: data_analysis, web_scraping, file_processing, etc."`
	Tags            string                           `v:"length:0,500" dc:"Tags (comma separated)"`
	RuntimeType     string                           `v:"required|in:python,node,shell" dc:"Runtime type: python, node, shell"`
	RuntimeVersion  string                           `v:"length:0,50" dc:"Runtime version (e.g., 3.9+)"`
	Requirements    []string                         `dc:"Dependencies list (e.g., pandas==2.0.0)"`
	ToolName        string                           `v:"required|length:1,100" dc:"Tool name"`
	ToolDescription string                           `v:"length:0,500" dc:"Tool description"`
	ToolParameters  map[string]SkillToolParameterDef `dc:"Tool parameters definition"`
	Script          string                           `v:"required" dc:"Script content"`
	IsPublic        bool                             `d:"false" dc:"Whether the skill is public"`
	Metadata        map[string]interface{}           `dc:"Additional metadata"`
}

// SkillToolParameterDef 工具参数定义
type SkillToolParameterDef struct {
	Type        string      `json:"type" dc:"Parameter type: string, number, boolean, array, object"`
	Required    bool        `json:"required" dc:"Whether the parameter is required"`
	Description string      `json:"description" dc:"Parameter description"`
	Default     interface{} `json:"default,omitempty" dc:"Default value"`
}

type SkillCreateRes struct {
	Id string `json:"id" dc:"Skill ID"`
}

// SkillUpdateReq 更新 Skill 请求
type SkillUpdateReq struct {
	g.Meta          `path:"/v1/skills/{id}" method:"put" tags:"skills" summary:"Update a skill"`
	Id              string                            `v:"required" dc:"Skill ID"`
	Name            *string                           `v:"length:1,100" dc:"Skill name"`
	Description     *string                           `v:"length:0,500" dc:"Skill description"`
	Version         *string                           `v:"length:0,50" dc:"Version"`
	Author          *string                           `v:"length:0,100" dc:"Author name"`
	Category        *string                           `v:"length:0,50" dc:"Category"`
	Tags            *string                           `v:"length:0,500" dc:"Tags"`
	RuntimeType     *string                           `v:"in:python,node,shell" dc:"Runtime type"`
	RuntimeVersion  *string                           `v:"length:0,50" dc:"Runtime version"`
	Requirements    []string                          `dc:"Dependencies list"`
	ToolName        *string                           `v:"length:1,100" dc:"Tool name"`
	ToolDescription *string                           `v:"length:0,500" dc:"Tool description"`
	ToolParameters  *map[string]SkillToolParameterDef `dc:"Tool parameters"`
	Script          *string                           `dc:"Script content"`
	Status          *int8                             `v:"in:0,1" dc:"Status: 1-enabled, 0-disabled"`
	IsPublic        *bool                             `dc:"Whether the skill is public"`
	Metadata        *map[string]interface{}           `dc:"Additional metadata"`
}

type SkillUpdateRes struct{}

// SkillDeleteReq 删除 Skill 请求
type SkillDeleteReq struct {
	g.Meta `path:"/v1/skills/{id}" method:"delete" tags:"skills" summary:"Delete a skill"`
	Id     string `v:"required" dc:"Skill ID"`
}

type SkillDeleteRes struct{}

// SkillGetOneReq 获取单个 Skill 请求
type SkillGetOneReq struct {
	g.Meta `path:"/v1/skills/{id}" method:"get" tags:"skills" summary:"Get one skill"`
	Id     string `v:"required" dc:"Skill ID"`
}

type SkillGetOneRes struct {
	Id              string                           `json:"id" dc:"Skill ID"`
	Name            string                           `json:"name" dc:"Skill name"`
	Description     string                           `json:"description" dc:"Skill description"`
	Version         string                           `json:"version" dc:"Version"`
	Author          string                           `json:"author" dc:"Author"`
	Category        string                           `json:"category" dc:"Category"`
	Tags            string                           `json:"tags" dc:"Tags"`
	RuntimeType     string                           `json:"runtime_type" dc:"Runtime type"`
	RuntimeVersion  string                           `json:"runtime_version" dc:"Runtime version"`
	Requirements    []string                         `json:"requirements" dc:"Dependencies"`
	ToolName        string                           `json:"tool_name" dc:"Tool name"`
	ToolDescription string                           `json:"tool_description" dc:"Tool description"`
	ToolParameters  map[string]SkillToolParameterDef `json:"tool_parameters" dc:"Tool parameters"`
	Script          string                           `json:"script" dc:"Script content"`
	ScriptHash      string                           `json:"script_hash" dc:"Script hash"`
	Metadata        map[string]interface{}           `json:"metadata,omitempty" dc:"Metadata"`
	CallCount       int64                            `json:"call_count" dc:"Call count"`
	SuccessCount    int64                            `json:"success_count" dc:"Success count"`
	FailCount       int64                            `json:"fail_count" dc:"Fail count"`
	AvgDuration     int64                            `json:"avg_duration" dc:"Average duration (ms)"`
	LastUsedAt      string                           `json:"last_used_at,omitempty" dc:"Last used time"`
	Status          int8                             `json:"status" dc:"Status"`
	IsPublic        bool                             `json:"is_public" dc:"Is public"`
	OwnerID         string                           `json:"owner_id" dc:"Owner ID"`
	CreateTime      string                           `json:"create_time" dc:"Create time"`
	UpdateTime      string                           `json:"update_time" dc:"Update time"`
}

// SkillGetListReq 获取 Skill 列表请求
type SkillGetListReq struct {
	g.Meta        `path:"/v1/skills" method:"get" tags:"skills" summary:"Get skills list"`
	Status        *int8  `v:"in:0,1" dc:"Status filter: 1-enabled, 0-disabled"`
	Category      string `dc:"Category filter"`
	IncludePublic bool   `d:"true" dc:"Include public skills"`
	PublicOnly    bool   `d:"false" dc:"Only public skills"`
	Keyword       string `dc:"Search keyword (name, description, tags)"`
	OrderBy       string `dc:"Order by field (e.g., create_time DESC, call_count DESC)"`
	Page          int    `v:"min:1" d:"1" dc:"Page number"`
	PageSize      int    `v:"min:1|max:100" d:"10" dc:"Page size"`
}

type SkillGetListRes struct {
	List  []*SkillItem `json:"list" dc:"Skills list"`
	Total int64        `json:"total" dc:"Total count"`
	Page  int          `json:"page" dc:"Current page"`
}

type SkillItem struct {
	Id              string   `json:"id" dc:"Skill ID"`
	Name            string   `json:"name" dc:"Skill name"`
	Description     string   `json:"description" dc:"Skill description"`
	Version         string   `json:"version" dc:"Version"`
	Author          string   `json:"author" dc:"Author"`
	Category        string   `json:"category" dc:"Category"`
	Tags            string   `json:"tags" dc:"Tags"`
	RuntimeType     string   `json:"runtime_type" dc:"Runtime type"`
	Requirements    []string `json:"requirements" dc:"Dependencies"`
	ToolName        string   `json:"tool_name" dc:"Tool name"`
	ToolDescription string   `json:"tool_description" dc:"Tool description"`
	CallCount       int64    `json:"call_count" dc:"Call count"`
	SuccessCount    int64    `json:"success_count" dc:"Success count"`
	AvgDuration     int64    `json:"avg_duration" dc:"Average duration (ms)"`
	LastUsedAt      string   `json:"last_used_at,omitempty" dc:"Last used time"`
	Status          int8     `json:"status" dc:"Status"`
	IsPublic        bool     `json:"is_public" dc:"Is public"`
	OwnerID         string   `json:"owner_id" dc:"Owner ID"`
	CreateTime      string   `json:"create_time" dc:"Create time"`
	UpdateTime      string   `json:"update_time" dc:"Update time"`
}

// SkillExecuteReq 执行 Skill 请求
type SkillExecuteReq struct {
	g.Meta    `path:"/v1/skills/{id}/execute" method:"post" tags:"skills" summary:"Execute a skill"`
	Id        string                 `v:"required" dc:"Skill ID"`
	Arguments map[string]interface{} `v:"required" dc:"Execution arguments"`
}

type SkillExecuteRes struct {
	Success  bool   `json:"success" dc:"Execution success"`
	Output   string `json:"output,omitempty" dc:"Execution output"`
	Error    string `json:"error,omitempty" dc:"Error message"`
	Duration int64  `json:"duration" dc:"Execution duration (ms)"`
}

// SkillCallLogsReq 获取 Skill 调用日志请求
type SkillCallLogsReq struct {
	g.Meta         `path:"/v1/skills/{id}/logs" method:"get" tags:"skills" summary:"Get skill call logs"`
	Id             string `v:"required" dc:"Skill ID"`
	ConversationID string `dc:"Conversation ID filter"`
	Success        *bool  `dc:"Success status filter"`
	Page           int    `v:"min:1" d:"1" dc:"Page number"`
	PageSize       int    `v:"min:1|max:100" d:"10" dc:"Page size"`
}

type SkillCallLogsRes struct {
	List  []*SkillCallLogItem `json:"list" dc:"Call logs list"`
	Total int64               `json:"total" dc:"Total count"`
	Page  int                 `json:"page" dc:"Current page"`
}

type SkillCallLogItem struct {
	Id              string `json:"id" dc:"Log ID"`
	SkillID         string `json:"skill_id" dc:"Skill ID"`
	SkillName       string `json:"skill_name" dc:"Skill name"`
	ConversationID  string `json:"conversation_id,omitempty" dc:"Conversation ID"`
	MessageID       string `json:"message_id,omitempty" dc:"Message ID"`
	RequestPayload  string `json:"request_payload" dc:"Request payload"`
	ResponsePayload string `json:"response_payload,omitempty" dc:"Response payload"`
	Success         bool   `json:"success" dc:"Success status"`
	ErrorMessage    string `json:"error_message,omitempty" dc:"Error message"`
	Duration        int64  `json:"duration" dc:"Duration (ms)"`
	VenvHash        string `json:"venv_hash,omitempty" dc:"Virtual environment hash"`
	VenvCacheHit    bool   `json:"venv_cache_hit" dc:"Virtual environment cache hit"`
	CreateTime      string `json:"create_time" dc:"Create time"`
}

// SkillCategoriesReq 获取 Skill 分类列表请求
type SkillCategoriesReq struct {
	g.Meta `path:"/v1/skills/categories" method:"get" tags:"skills" summary:"Get skill categories"`
}

type SkillCategoriesRes struct {
	Categories []SkillCategoryItem `json:"categories" dc:"Categories list"`
}

type SkillCategoryItem struct {
	Name  string `json:"name" dc:"Category name"`
	Count int64  `json:"count" dc:"Skills count in this category"`
}
