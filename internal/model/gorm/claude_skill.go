package gorm

import (
	"time"
)

// ClaudeSkill Claude Skill 模型定义
type ClaudeSkill struct {
	ID          string `gorm:"primaryKey;column:id;type:varchar(64)"`        // Skill 唯一ID
	Name        string `gorm:"column:name;type:varchar(100);not null;index"` // Skill 名称
	Description string `gorm:"column:description;type:varchar(500)"`         // Skill 描述
	Version     string `gorm:"column:version;type:varchar(50)"`              // 版本号
	Author      string `gorm:"column:author;type:varchar(100)"`              // 作者
	Category    string `gorm:"column:category;type:varchar(50);index"`       // 分类：data_analysis, web_scraping, file_processing, etc.
	Tags        string `gorm:"column:tags;type:varchar(500)"`                // 标签（逗号分隔）

	// 运行时配置
	RuntimeType    string `gorm:"column:runtime_type;type:varchar(20);not null"` // python, node, shell
	RuntimeVersion string `gorm:"column:runtime_version;type:varchar(50)"`       // 3.9+, 18+
	Requirements   string `gorm:"column:requirements;type:text"`                 // 依赖列表（JSON数组）

	// 工具定义
	ToolName        string `gorm:"column:tool_name;type:varchar(100);not null"` // 工具名称
	ToolDescription string `gorm:"column:tool_description;type:varchar(500)"`   // 工具描述
	ToolParameters  string `gorm:"column:tool_parameters;type:text"`            // 工具参数（JSON格式）

	// 脚本内容
	Script     string `gorm:"column:script;type:text;not null"`          // 脚本内容
	ScriptHash string `gorm:"column:script_hash;type:varchar(64);index"` // 脚本哈希（用于去重）

	// 元数据
	Metadata string `gorm:"column:metadata;type:text"` // 额外元数据（JSON格式）

	// 使用统计
	CallCount    int64      `gorm:"column:call_count;default:0"`    // 调用次数
	SuccessCount int64      `gorm:"column:success_count;default:0"` // 成功次数
	FailCount    int64      `gorm:"column:fail_count;default:0"`    // 失败次数
	AvgDuration  int64      `gorm:"column:avg_duration;default:0"`  // 平均执行时间（毫秒）
	LastUsedAt   *time.Time `gorm:"column:last_used_at"`            // 最后使用时间

	// 状态管理
	Status   int8   `gorm:"column:status;default:1;index"`          // 状态：1-启用，0-禁用
	IsPublic bool   `gorm:"column:is_public;default:false"`         // 是否公开（可被其他用户使用）
	OwnerID  string `gorm:"column:owner_id;type:varchar(64);index"` // 所有者ID

	// 时间戳
	CreateTime *time.Time `gorm:"column:create_time;autoCreateTime"` // 创建时间
	UpdateTime *time.Time `gorm:"column:update_time;autoUpdateTime"` // 更新时间
}

// TableName 设置表名
func (ClaudeSkill) TableName() string {
	return "claude_skills"
}

// ClaudeSkillCallLog Skill 调用日志
type ClaudeSkillCallLog struct {
	ID             string `gorm:"primaryKey;column:id;type:varchar(64)"`           // 日志ID
	SkillID        string `gorm:"column:skill_id;type:varchar(64);not null;index"` // Skill ID
	SkillName      string `gorm:"column:skill_name;type:varchar(100)"`             // Skill 名称（冗余，便于查询）
	ConversationID string `gorm:"column:conversation_id;type:varchar(64);index"`   // 会话ID
	MessageID      string `gorm:"column:message_id;type:varchar(64);index"`        // 消息ID

	// 请求信息
	RequestPayload string `gorm:"column:request_payload;type:text"` // 请求参数（JSON）

	// 响应信息
	ResponsePayload string `gorm:"column:response_payload;type:text"` // 响应内容
	Success         bool   `gorm:"column:success;default:false"`      // 是否成功
	ErrorMessage    string `gorm:"column:error_message;type:text"`    // 错误信息

	// 性能指标
	Duration     int64  `gorm:"column:duration;default:0"`           // 执行时间（毫秒）
	VenvHash     string `gorm:"column:venv_hash;type:varchar(64)"`   // 使用的虚拟环境哈希
	VenvCacheHit bool   `gorm:"column:venv_cache_hit;default:false"` // 虚拟环境是否命中缓存

	// 时间戳
	CreateTime *time.Time `gorm:"column:create_time;autoCreateTime"` // 创建时间
}

// TableName 设置表名
func (ClaudeSkillCallLog) TableName() string {
	return "claude_skill_call_logs"
}
