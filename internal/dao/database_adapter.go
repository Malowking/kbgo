package dao

import (
	"context"
	"fmt"
	"time"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
)

// DBConfig 数据库配置
type DBConfig struct {
	Type string `json:"type"` // 数据库类型: pgsql, postgresql 或 postgres
	Host string `json:"host"` // 主机地址
	Port string `json:"port"` // 端口
	User string `json:"user"` // 用户名
	Pass string `json:"pass"` // 密码
	Name string `json:"name"` // 数据库名
}

// getDBConfig 从配置文件中获取数据库配置
// 支持通过环境变量覆盖配置文件中的值
func getDBConfig() *DBConfig {
	ctx := context.Background()

	// 先从配置文件读取
	dbType := g.Cfg().MustGet(ctx, "database.default.type").String()
	dbHost := g.Cfg().MustGet(ctx, "database.default.host").String()
	dbPort := g.Cfg().MustGet(ctx, "database.default.port").String()
	dbUser := g.Cfg().MustGet(ctx, "database.default.user").String()
	dbPass := g.Cfg().MustGet(ctx, "database.default.pass").String()
	dbName := g.Cfg().MustGet(ctx, "database.default.name").String()

	g.Log().Infof(ctx, "Config from file - type:%s, host:%s, port:%s, user:%s, name:%s",
		dbType, dbHost, dbPort, dbUser, dbName)

	return &DBConfig{
		Type: dbType,
		Host: dbHost,
		Port: dbPort,
		User: dbUser,
		Pass: dbPass,
		Name: dbName,
	}
}

// buildDSN 构建数据库连接字符串
func buildDSN(config *DBConfig) (string, error) {
	switch config.Type {
	case "postgresql", "postgres", "pgsql":
		// 构建 PostgreSQL DSN，如果密码为空则不包含 password 参数
		dsn := fmt.Sprintf("host=%s user=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai",
			config.Host, config.User, config.Name, config.Port)

		// 只有当密码非空时才添加 password 参数
		if config.Pass != "" {
			dsn = fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai",
				config.Host, config.User, config.Pass, config.Name, config.Port)
		}

		return dsn, nil
	default:
		return "", errors.Newf(errors.ErrInvalidParameter, "unsupported database type: %s", config.Type)
	}
}

// initDatabase 根据配置初始化数据库连接
func initDatabase() (*gorm.DB, error) {
	config := getDBConfig()

	// 构建 DSN
	dsn, err := buildDSN(config)
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseInit, "failed to build DSN: %v", err)
	}

	// 打印 DSN 用于调试
	g.Log().Infof(context.Background(), "DSN: %s", dsn)

	// GORM 配置
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	var db *gorm.DB

	// 根据数据库类型选择对应的驱动
	switch config.Type {
	case "postgresql", "postgres", "pgsql":
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	default:
		return nil, errors.Newf(errors.ErrInvalidParameter, "unsupported database type: %s", config.Type)
	}

	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseInit, "failed to connect database: %v", err)
	}

	// 设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Newf(errors.ErrDatabaseInit, "failed to get database instance: %v", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour) // 使用固定的1小时连接最大生命周期

	// 自动迁移数据库表结构
	if err = gormModel.Migrate(db); err != nil {
		return nil, errors.Newf(errors.ErrDatabaseInit, "failed to migrate database tables: %v", err)
	}

	return db, nil
}
