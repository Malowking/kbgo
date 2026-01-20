package datasource

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"

	"github.com/Malowking/kbgo/nl2sql/common"
)

// DBConfig 数据库连接配置
type DBConfig struct {
	DBType   string `json:"db_type"` // postgresql
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"` // disable, require
}

// JDBCConnector JDBC连接器
type JDBCConnector struct {
	config *DBConfig
	db     *sql.DB
}

// NewJDBCConnector 创建JDBC连接器
func NewJDBCConnector(config *DBConfig) *JDBCConnector {
	return &JDBCConnector{
		config: config,
	}
}

// NewJDBCDataSource 从通用Config创建JDBC数据源（实现工厂模式）
func NewJDBCDataSource(config *Config) (DataSource, error) {
	// 从 config.Settings 中提取配置信息
	dbConfig := &DBConfig{
		DBType: config.DBType,
	}

	// 解析 Settings 到 DBConfig
	if host, ok := config.Settings["host"].(string); ok {
		dbConfig.Host = host
	}
	if port, ok := config.Settings["port"].(int); ok {
		dbConfig.Port = port
	} else if portFloat, ok := config.Settings["port"].(float64); ok {
		dbConfig.Port = int(portFloat)
	}
	if database, ok := config.Settings["database"].(string); ok {
		dbConfig.Database = database
	}
	if username, ok := config.Settings["username"].(string); ok {
		dbConfig.Username = username
	}
	if password, ok := config.Settings["password"].(string); ok {
		dbConfig.Password = password
	}
	if sslMode, ok := config.Settings["ssl_mode"].(string); ok {
		dbConfig.SSLMode = sslMode
	} else {
		dbConfig.SSLMode = "disable" // 默认值
	}

	return NewJDBCConnector(dbConfig), nil
}

// Connect 连接数据库
func (c *JDBCConnector) Connect(ctx context.Context) error {
	dsn := c.buildDSN()

	var err error
	// 获取正确的驱动名称
	driverName := c.getDriverName()
	c.db, err = sql.Open(driverName, dsn)
	if err != nil {
		return fmt.Errorf("打开数据库连接失败: %w", err)
	}

	// 设置连接池参数
	c.db.SetMaxOpenConns(10)
	c.db.SetMaxIdleConns(5)
	c.db.SetConnMaxLifetime(time.Hour)

	// 测试连接
	if err := c.db.PingContext(ctx); err != nil {
		return fmt.Errorf("数据库连接测试失败: %w", err)
	}

	return nil
}

// getDriverName 获取正确的数据库驱动名称
func (c *JDBCConnector) getDriverName() string {
	switch c.config.DBType {
	case common.DBTypePostgreSQL:
		return "postgres" // lib/pq 驱动注册的名称
	default:
		return c.config.DBType
	}
}

// buildDSN 构建DSN字符串
func (c *JDBCConnector) buildDSN() string {
	switch c.config.DBType {
	case common.DBTypePostgreSQL:
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.config.Host,
			c.config.Port,
			c.config.Username,
			c.config.Password,
			c.config.Database,
			c.config.SSLMode,
		)
	default:
		return ""
	}
}

// Close 关闭连接
func (c *JDBCConnector) Close() error {
	if c.db != nil {
		return c.db.Close()
	}
	return nil
}

// TestConnection 测试连接（验证只读权限）
func (c *JDBCConnector) TestConnection(ctx context.Context) error {
	// 1. 尝试执行一个简单的SELECT查询
	_, err := c.db.QueryContext(ctx, "SELECT 1")
	if err != nil {
		return fmt.Errorf("测试查询失败: %w", err)
	}

	// 2. 尝试执行写操作（应该失败）
	testTable := fmt.Sprintf("_nl2sql_write_test_%d", time.Now().Unix())
	_, err = c.db.ExecContext(ctx, fmt.Sprintf("CREATE TABLE %s (id INT)", testTable))
	if err == nil {
		// 如果成功创建表，说明有写权限，清理并返回错误
		_, _ = c.db.ExecContext(ctx, fmt.Sprintf("DROP TABLE %s", testTable))
		return fmt.Errorf("数据库账号具有写权限，请使用只读账号")
	}

	return nil
}

// GetTables 获取所有表名
func (c *JDBCConnector) GetTables(ctx context.Context) ([]string, error) {
	var query string

	switch c.config.DBType {
	case common.DBTypePostgreSQL:
		query = `
			SELECT table_name
			FROM information_schema.tables
			WHERE table_schema = 'public'
			  AND table_type = 'BASE TABLE'
			ORDER BY table_name
		`
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", c.config.DBType)
	}

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询表列表失败: %w", err)
	}
	defer rows.Close()

	tables := make([]string, 0)
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("扫描表名失败: %w", err)
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

// IndexSchema 索引结构定义
type IndexSchema struct {
	IndexName string
	Columns   []string
	IsUnique  bool
}

// GetTableSchema 获取表结构
func (c *JDBCConnector) GetTableSchema(ctx context.Context, tableName string) (*TableSchema, error) {
	schema := &TableSchema{
		TableName: tableName,
	}

	// 1. 获取列信息
	columns, err := c.getColumns(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.Columns = columns

	// 2. 获取主键信息
	primaryKeys, err := c.getPrimaryKeys(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.PrimaryKeys = primaryKeys

	// 3. 获取索引信息（暂时跳过，datasource.TableSchema没有Indexes字段）
	// indexes, err := c.getIndexes(ctx, tableName)
	// if err != nil {
	// 	return nil, err
	// }
	// schema.Indexes = indexes

	// 4. 获取行数估算
	rowCount, err := c.getRowCount(ctx, tableName)
	if err != nil {
		return nil, err
	}
	schema.RowCount = rowCount

	return schema, nil
}

// getColumns 获取列信息
func (c *JDBCConnector) getColumns(ctx context.Context, tableName string) ([]ColumnSchema, error) {
	var query string

	switch c.config.DBType {
	case common.DBTypePostgreSQL:
		query = `
			SELECT
				column_name,
				data_type,
				is_nullable,
				column_default,
				NULL as column_comment
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = $1
			ORDER BY ordinal_position
		`
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", c.config.DBType)
	}

	rows, err := c.db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("查询列信息失败: %w", err)
	}
	defer rows.Close()

	columns := make([]ColumnSchema, 0)
	for rows.Next() {
		var col ColumnSchema
		var isNullable string
		var comment *string
		if err := rows.Scan(&col.Name, &col.DataType, &isNullable, &col.DefaultValue, &comment); err != nil {
			return nil, fmt.Errorf("扫描列信息失败: %w", err)
		}
		col.Nullable = (isNullable == "YES")
		if comment != nil {
			col.Comment = *comment
		}
		columns = append(columns, col)
	}

	return columns, rows.Err()
}

// getPrimaryKeys 获取主键信息
func (c *JDBCConnector) getPrimaryKeys(ctx context.Context, tableName string) ([]string, error) {
	var query string

	switch c.config.DBType {
	case common.DBTypePostgreSQL:
		query = `
			SELECT a.attname
			FROM pg_index i
			JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
			WHERE i.indrelid = $1::regclass
			  AND i.indisprimary
		`
	default:
		return nil, fmt.Errorf("不支持的数据库类型: %s", c.config.DBType)
	}

	rows, err := c.db.QueryContext(ctx, query, tableName)
	if err != nil {
		return nil, fmt.Errorf("查询主键失败: %w", err)
	}
	defer rows.Close()

	primaryKeys := make([]string, 0)
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, fmt.Errorf("扫描主键失败: %w", err)
		}
		primaryKeys = append(primaryKeys, columnName)
	}

	return primaryKeys, rows.Err()
}

// getIndexes 获取索引信息
func (c *JDBCConnector) getIndexes(ctx context.Context, tableName string) ([]IndexSchema, error) {
	// 简化实现：只返回空索引列表
	// 完整实现需要针对不同数据库解析索引信息
	return []IndexSchema{}, nil
}

// getRowCount 获取行数估算
func (c *JDBCConnector) getRowCount(ctx context.Context, tableName string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)

	err := c.db.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("查询行数失败: %w", err)
	}

	return count, nil
}

// SampleRows 采样数据
func (c *JDBCConnector) SampleRows(ctx context.Context, tableName string, limit int) ([]map[string]interface{}, error) {
	query := fmt.Sprintf("SELECT * FROM %s LIMIT %d", tableName, limit)

	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询采样数据失败: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("获取列名失败: %w", err)
	}

	results := make([]map[string]interface{}, 0)
	for rows.Next() {
		// 创建扫描目标
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("扫描行数据失败: %w", err)
		}

		// 转换为map
		row := make(map[string]interface{})
		for i, colName := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				row[colName] = string(b)
			} else {
				row[colName] = val
			}
		}

		results = append(results, row)
	}

	return results, rows.Err()
}

// ExecuteQuery 执行查询（实现DataSource接口）
func (c *JDBCConnector) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	rows, err := c.db.QueryContext(ctx, query)
	if err != nil {
		return &QueryResult{Error: fmt.Errorf("执行查询失败: %w", err)}, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return &QueryResult{Error: fmt.Errorf("获取列名失败: %w", err)}, err
	}

	result := &QueryResult{
		Columns: columns,
		Rows:    make([][]interface{}, 0),
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return &QueryResult{Error: fmt.Errorf("扫描行数据失败: %w", err)}, err
		}

		// 转换字节数组为字符串
		rowData := make([]interface{}, len(columns))
		for i, val := range values {
			if b, ok := val.([]byte); ok {
				rowData[i] = string(b)
			} else {
				rowData[i] = val
			}
		}

		result.Rows = append(result.Rows, rowData)
	}

	if err := rows.Err(); err != nil {
		result.Error = err
		return result, err
	}

	return result, nil
}
