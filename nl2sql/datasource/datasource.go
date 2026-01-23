package datasource

import (
	"context"
	"fmt"

	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"
)

// DataSource 数据源接口
type DataSource interface {
	// Connect 连接数据源
	Connect(ctx context.Context) error
	// Close 关闭连接
	Close() error
	// GetTables 获取所有表名
	GetTables(ctx context.Context) ([]string, error)
	// GetTableSchema 获取表结构
	GetTableSchema(ctx context.Context, tableName string) (*TableSchema, error)
	// ExecuteQuery 执行查询
	ExecuteQuery(ctx context.Context, query string) (*QueryResult, error)
	// TestConnection 测试连接
	TestConnection(ctx context.Context) error
}

// TableSchema 表结构
type TableSchema struct {
	TableName   string
	Columns     []ColumnSchema
	PrimaryKeys []string
	ForeignKeys []ForeignKey
	RowCount    int64
}

// ColumnSchema 列结构
type ColumnSchema struct {
	Name         string
	DataType     string
	Nullable     bool
	DefaultValue *string
	Comment      string
	IsPrimaryKey bool
	IsForeignKey bool
}

// ForeignKey 外键关系
type ForeignKey struct {
	ColumnName     string
	RefTableName   string
	RefColumnName  string
	ConstraintName string
}

// QueryResult 查询结果
type QueryResult struct {
	Columns []string
	Rows    [][]interface{}
	Error   error
}

// Config 数据源配置
type Config struct {
	Type     string                 // 'jdbc', 'csv', 'excel'
	DBType   string                 // 'postgresql' (for JDBC)
	Settings map[string]interface{} // 具体配置
}

// DataSourceFactory 数据源工厂
type DataSourceFactory struct{}

// NewDataSourceFactory 创建工厂
func NewDataSourceFactory() *DataSourceFactory {
	return &DataSourceFactory{}
}

// Create 创建数据源
func (f *DataSourceFactory) Create(config *Config) (DataSource, error) {
	switch config.Type {
	case nl2sqlCommon.DataSourceTypeJDBC:
		return NewJDBCDataSource(config)
	case nl2sqlCommon.DataSourceTypeCSV:
		return NewCSVDataSource(config)
	case nl2sqlCommon.DataSourceTypeExcel:
		return NewExcelDataSource(config)
	default:
		return nil, fmt.Errorf("不支持的数据源类型: %s", config.Type)
	}
}
