package gorm

import (
	"context"

	"github.com/gogf/gf/v2/os/glog"
	"gorm.io/gorm"
)

// Migrate 数据库迁移
func Migrate(db *gorm.DB) error {
	err := db.AutoMigrate(
		&User{},
		&Conversation{},
		&Message{},
		&MessageContent{},
		&KnowledgeBase{},
		&KnowledgeDocuments{},
		&KnowledgeChunks{},
		&MCPRegistry{},
		&MCPCallLog{},
	)
	if err != nil {
		glog.Error(context.Background(), "数据库迁移失败:", err)
		return err
	}
	glog.Info(context.Background(), "数据库迁移成功")
	return nil
}
