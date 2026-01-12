package gorm

import (
	"time"
)

// MessageContent 消息内容块表
type MessageContent struct {
	ID          uint       `gorm:"primaryKey;autoIncrement;column:id"`                  // 主键ID（自增）
	MsgID       string     `gorm:"column:msg_id;type:varchar(64);not null;index"`       // 消息ID（外键引用 messages.msg_id）
	ContentType string     `gorm:"column:content_type;type:varchar(32);not null;index"` // 内容类型
	TextContent string     `gorm:"column:text_content;type:text"`                       // 文本内容
	MediaURL    string     `gorm:"column:media_url;type:varchar(512)"`                  // 媒体URL
	Metadata    JSON       `gorm:"column:metadata;type:json"`                           // 元数据
	SortOrder   int        `gorm:"column:sort_order;type:int;default:0"`                // 排序
	CreateTime  *time.Time `gorm:"column:create_time;autoCreateTime"`                   // 创建时间
}

// TableName 设置表名
func (MessageContent) TableName() string {
	return "message_contents"
}
