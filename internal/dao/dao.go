package dao

import (
	"context"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

var db *gorm.DB

// InitDB 初始化数据库连接
func InitDB() error {
	var err error
	db, err = initDatabase()
	if err != nil {
		return err
	}
	return nil
}

// GetDB 获取数据库实例
func GetDB() *gorm.DB {
	if db == nil {
		g.Log().Fatal(context.Background(), "database connection not initialized")
	}
	return db
}
