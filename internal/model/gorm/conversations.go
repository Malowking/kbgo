package gorm

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Conversation 会话表
type Conversation struct {
	ConvID           string     `gorm:"primaryKey;column:conv_id;type:varchar(64)"`               // 会话ID（主键，格式：conv_uuid）
	UserID           string     `gorm:"column:user_id;type:varchar(64);not null;index"`           // 用户ID
	Title            string     `gorm:"column:title;type:varchar(255)"`                           // 会话标题
	ModelID          string     `gorm:"column:model_id;type:varchar(64)"`                         // 模型ID
	ConversationType string     `gorm:"column:conversation_type;type:varchar(32);default:'text'"` // 会话类型
	Status           string     `gorm:"column:status;type:varchar(20);default:'active'"`          // 状态
	AgentPresetID    string     `gorm:"column:agent_preset_id;type:varchar(64);index"`            // 关联的Agent预设ID
	Metadata         JSON       `gorm:"column:metadata;type:json"`                                // 扩展元数据
	CreateTime       *time.Time `gorm:"column:create_time;autoCreateTime"`                        // 创建时间
	UpdateTime       *time.Time `gorm:"column:update_time;autoUpdateTime"`                        // 更新时间
}

// TableName 设置表名
func (Conversation) TableName() string {
	return "conversations"
}

// BeforeCreate GORM钩子：创建前自动生成ConvID
func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ConvID == "" {
		// 生成格式：conv_uuid（无连接符）
		uuidStr := uuid.New().String()
		uuidStr = uuidStr[:8] + uuidStr[9:13] + uuidStr[14:18] + uuidStr[19:23] + uuidStr[24:]
		c.ConvID = fmt.Sprintf("conv_%s", uuidStr)
	}
	return nil
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
