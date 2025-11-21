package gorm

import (
	"time"
)

// User 用户表
type User struct {
	ID         uint64     `gorm:"primaryKey;column:id;type:bigint"`
	UserID     string     `gorm:"column:user_id;type:varchar(64);uniqueIndex;not null"` // 业务ID
	Name       string     `gorm:"column:name;type:varchar(128)"`                        // 用户名
	CreateTime *time.Time `gorm:"column:create_time"`                                   // 创建时间
}

// TableName 设置表名
func (User) TableName() string {
	return "users"
}
