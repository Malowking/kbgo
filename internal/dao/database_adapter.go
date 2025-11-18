package dao

import (
	"fmt"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
)

// DBConfig 数据库配置
type DBConfig struct {
	Type    string `json:"type"`    // 数据库类型: mysql 或 postgresql
	Host    string `json:"host"`    // 主机地址
	Port    string `json:"port"`    // 端口
	User    string `json:"user"`    // 用户名
	Pass    string `json:"pass"`    // 密码
	Name    string `json:"name"`    // 数据库名
	Charset string `json:"charset"` // 字符集 (主要用于 MySQL)
}

// getDBConfig 从配置文件中获取数据库配置
func getDBConfig() *DBConfig {
	cfg := g.DB().GetConfig()
	return &DBConfig{
		Type:    cfg.Type,
		Host:    cfg.Host,
		Port:    cfg.Port,
		User:    cfg.User,
		Pass:    cfg.Pass,
		Name:    cfg.Name,
		Charset: cfg.Charset,
	}
}

// buildDSN 构建数据库连接字符串
func buildDSN(config *DBConfig) (string, error) {
	switch config.Type {
	case "mysql":
		return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=%s&parseTime=True&loc=Local",
			config.User, config.Pass, config.Host, config.Port, config.Name, config.Charset), nil
	case "postgresql", "postgres":
		return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Shanghai",
			config.Host, config.User, config.Pass, config.Name, config.Port), nil
	default:
		return "", fmt.Errorf("unsupported database type: %s", config.Type)
	}
}

// initDatabase 根据配置初始化数据库连接
func initDatabase() (*gorm.DB, error) {
	config := getDBConfig()

	// 构建 DSN
	dsn, err := buildDSN(config)
	if err != nil {
		return nil, fmt.Errorf("failed to build DSN: %v", err)
	}

	// GORM 配置
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	}

	var db *gorm.DB

	// 根据数据库类型选择对应的驱动
	switch config.Type {
	case "mysql":
		db, err = gorm.Open(mysql.Open(dsn), gormConfig)
	case "postgresql", "postgres":
		db, err = gorm.Open(postgres.Open(dsn), gormConfig)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %v", err)
	}

	// 设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %v", err)
	}

	// 设置连接池
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour) // 使用固定的1小时连接最大生命周期

	// 自动迁移数据库表结构
	if err = gormModel.Migrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database tables: %v", err)
	}

	return db, nil
}
