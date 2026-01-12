package gorm

import (
	"github.com/gogf/gf/v2/os/gctx"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"gorm.io/gorm"
)

// InitNL2SQLSchema 初始化NL2SQL schema（用于存储CSV/Excel解析的表）
func InitNL2SQLSchema(db *gorm.DB) error {
	ctx := gctx.New()

	// 获取数据库类型
	dbType := g.Cfg().MustGet(ctx, "database.default.type").String()

	switch dbType {
	case "postgresql", "postgres", "pgsql":
		// PostgreSQL: 创建 nl2sql schema
		if err := db.Exec("CREATE SCHEMA IF NOT EXISTS nl2sql").Error; err != nil {
			glog.Error(ctx, "Failed to create nl2sql schema:", err)
			return err
		}
		glog.Info(ctx, "✓ NL2SQL schema initialized (PostgreSQL)")

	default:
		glog.Warningf(ctx, "Unknown database type: %s, skipping NL2SQL schema initialization", dbType)
	}

	return nil
}

// Migrate 数据库迁移
func Migrate(db *gorm.DB) error {
	// 1. 初始化 NL2SQL schema/database（用于存储CSV/Excel解析的表）
	if err := InitNL2SQLSchema(db); err != nil {
		glog.Warning(gctx.New(), "NL2SQL schema initialization failed:", err)
		// 不返回错误，继续迁移其他表
	}

	// 2. 自动迁移模型表
	err := db.AutoMigrate(
		// 现有模型
		&User{},
		&Conversation{},
		&Message{},
		&MessageContent{},
		&KnowledgeBase{},
		&KnowledgeDocuments{},
		&KnowledgeChunks{},
		&MCPRegistry{},
		&MCPCallLog{},
		&AIModel{},
		&AgentPreset{},

		// Claude Skills 模型
		&ClaudeSkill{},
		&ClaudeSkillCallLog{},

		// NL2SQL 模型
		&NL2SQLDataSource{}, // 数据源
		&NL2SQLMetric{},     // 指标
		&NL2SQLTable{},      // 表元数据
		&NL2SQLColumn{},     // 字段元数据
		&NL2SQLRelation{},   // 表关系
		&NL2SQLQueryLog{},   // 查询日志
		&NL2SQLVectorDoc{},  // 向量索引关联
	)
	if err != nil {
		glog.Error(gctx.New(), "数据库迁移失败:", err)
		return err
	}

	// 创建复合索引（AutoMigrate不会自动创建）
	createIndexes(db)

	glog.Info(gctx.New(), "数据库迁移成功")
	return nil
}

// createIndexes 创建额外的索引
func createIndexes(db *gorm.DB) {
	indexes := []string{
		// DataSource索引
		"CREATE INDEX IF NOT EXISTS idx_datasource_type ON nl2sql_datasources(type)",
		"CREATE INDEX IF NOT EXISTS idx_datasource_status ON nl2sql_datasources(status)",

		// Metric索引
		"CREATE INDEX IF NOT EXISTS idx_metric_datasource ON nl2sql_metrics(datasource_id)",

		// Table索引
		"CREATE INDEX IF NOT EXISTS idx_table_datasource ON nl2sql_tables(datasource_id)",
		"CREATE INDEX IF NOT EXISTS idx_table_parsed ON nl2sql_tables(parsed)",

		// Column索引
		"CREATE INDEX IF NOT EXISTS idx_column_table ON nl2sql_columns(table_id)",

		// Relation索引
		"CREATE INDEX IF NOT EXISTS idx_relation_datasource ON nl2sql_relations(datasource_id)",

		// QueryLog索引
		"CREATE INDEX IF NOT EXISTS idx_query_log_datasource ON nl2sql_query_logs(datasource_id)",
		"CREATE INDEX IF NOT EXISTS idx_query_log_msg ON nl2sql_query_logs(msg_id)",
		"CREATE INDEX IF NOT EXISTS idx_query_log_conv ON nl2sql_query_logs(conv_id)",
		"CREATE INDEX IF NOT EXISTS idx_query_log_status ON nl2sql_query_logs(execution_status)",
		"CREATE INDEX IF NOT EXISTS idx_query_log_time ON nl2sql_query_logs(execution_time_ms)",
		"CREATE INDEX IF NOT EXISTS idx_query_log_created ON nl2sql_query_logs(create_time DESC)",

		// VectorDoc索引
		"CREATE INDEX IF NOT EXISTS idx_vector_docs_datasource ON nl2sql_vector_docs(datasource_id)",
		"CREATE INDEX IF NOT EXISTS idx_vector_docs_entity ON nl2sql_vector_docs(entity_type, entity_id)",
	}

	for _, sql := range indexes {
		if err := db.Exec(sql).Error; err != nil {
			glog.Warning(gctx.New(), "创建索引失败:", sql, err)
		}
	}
}
