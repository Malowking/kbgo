package gorm

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

// Conversation 会话表
type Conversation struct {
	ID               uint64     `gorm:"primaryKey;column:id;type:bigint"`
	ConvID           string     `gorm:"column:conv_id;type:varchar(64);uniqueIndex;not null"`     // 会话ID
	UserID           string     `gorm:"column:user_id;type:varchar(64);not null;index"`           // 用户ID
	Title            string     `gorm:"column:title;type:varchar(255)"`                           // 会话标题
	ModelName        string     `gorm:"column:model_name;type:varchar(64);not null"`              // 模型名称
	ConversationType string     `gorm:"column:conversation_type;type:varchar(32);default:'text'"` // 会话类型
	Status           string     `gorm:"column:status;type:varchar(20);default:'active'"`          // 状态
	AgentPresetID    string     `gorm:"column:agent_preset_id;type:varchar(64);index"`            // 关联的Agent预设ID
	Metadata         JSON       `gorm:"column:metadata;type:json"`                                // 扩展元数据
	CreateTime       *time.Time `gorm:"column:create_time"`                                       // 创建时间
	UpdateTime       *time.Time `gorm:"column:update_time"`                                       // 更新时间
}

// TableName 设置表名
func (Conversation) TableName() string {
	return "conversations"
}

// JSON 自定义JSON类型
type JSON json.RawMessage

// Scan 实现sql.Scanner接口
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = JSON("null")
		return nil
	}
	var bytes []byte
	switch v := value.(type) {
	case []byte:
		bytes = v
	case string:
		bytes = []byte(v)
	default:
		return nil
	}
	*j = JSON(bytes)
	return nil
}

// Value 实现driver.Valuer接口
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return nil, nil
	}
	return string(j), nil
}
