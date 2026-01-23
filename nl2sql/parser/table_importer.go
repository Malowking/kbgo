package parser

import (
	"context"
	"fmt"
	"strings"

	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// TableImporter 表导入器
type TableImporter struct {
	db     *gorm.DB
	dbType string // postgresql
}

// NewTableImporter 创建表导入器
func NewTableImporter(db *gorm.DB, dbType string) *TableImporter {
	return &TableImporter{
		db:     db,
		dbType: dbType,
	}
}

// ImportTable 将解析的表导入到nl2sql schema/database
func (t *TableImporter) ImportTable(ctx context.Context, table *ParsedTable) error {
	g.Log().Infof(ctx, "Importing table: %s (%d columns, %d rows)",
		table.TableName, len(table.Columns), len(table.Rows))

	// 1. 创建表
	if err := t.createTable(ctx, table); err != nil {
		return fmt.Errorf("failed to create table: %w", err)
	}

	// 2. 插入数据
	if err := t.insertData(ctx, table); err != nil {
		return fmt.Errorf("failed to insert data: %w", err)
	}

	g.Log().Infof(ctx, "Table imported successfully: %s", table.TableName)
	return nil
}

// createTable 创建表结构
func (t *TableImporter) createTable(ctx context.Context, table *ParsedTable) error {
	var createSQL string

	switch t.dbType {
	case nl2sqlCommon.DBTypePostgreSQL, "postgres", "pgsql":
		// PostgreSQL: 在 nl2sql schema 中创建表
		createSQL = t.buildPostgreSQLCreateTable(table)
	default:
		return fmt.Errorf("unsupported database type: %s", t.dbType)
	}

	// 执行创建表SQL
	if err := t.db.Exec(createSQL).Error; err != nil {
		return fmt.Errorf("failed to execute create table: %w", err)
	}

	return nil
}

// buildPostgreSQLCreateTable 构建PostgreSQL创建表语句
func (t *TableImporter) buildPostgreSQLCreateTable(table *ParsedTable) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS nl2sql.%s (\n", table.TableName))
	sb.WriteString("  id SERIAL PRIMARY KEY,\n")

	for i, col := range table.Columns {
		sb.WriteString(fmt.Sprintf("  %s %s", col.Name, mapTypeToPg(col.DataType)))
		if !col.Nullable {
			sb.WriteString(" NOT NULL")
		}
		if i < len(table.Columns)-1 {
			sb.WriteString(",\n")
		} else {
			sb.WriteString("\n")
		}
	}

	sb.WriteString(");")
	return sb.String()
}

// mapTypeToPg 映射类型到PostgreSQL类型
func mapTypeToPg(dataType string) string {
	switch dataType {
	case "INTEGER":
		return "BIGINT"
	case "FLOAT":
		return "DOUBLE PRECISION"
	case "BOOLEAN":
		return "BOOLEAN"
	case "TIMESTAMP":
		return "TIMESTAMP"
	default:
		return "TEXT"
	}
}

// insertData 插入数据
func (t *TableImporter) insertData(ctx context.Context, table *ParsedTable) error {
	if len(table.Rows) == 0 {
		g.Log().Warning(ctx, "No data to insert")
		return nil
	}

	// 构建批量插入SQL
	batchSize := 500
	for i := 0; i < len(table.Rows); i += batchSize {
		end := i + batchSize
		if end > len(table.Rows) {
			end = len(table.Rows)
		}

		batch := table.Rows[i:end]
		if err := t.insertBatch(ctx, table, batch); err != nil {
			return fmt.Errorf("failed to insert batch at row %d: %w", i, err)
		}

		g.Log().Infof(ctx, "Inserted rows %d-%d of %d", i+1, end, len(table.Rows))
	}

	return nil
}

// insertBatch 批量插入数据
func (t *TableImporter) insertBatch(ctx context.Context, table *ParsedTable, rows [][]interface{}) error {
	if len(rows) == 0 {
		return nil
	}

	// 构建列名
	var columnNames []string
	for _, col := range table.Columns {
		columnNames = append(columnNames, col.Name)
	}

	// 构建INSERT语句
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("INSERT INTO nl2sql.%s (%s) VALUES\n",
		table.TableName, strings.Join(columnNames, ", ")))

	// 构建VALUES部分
	var values []interface{}
	for i, row := range rows {
		sb.WriteString("(")
		for j := range table.Columns {
			if j < len(row) {
				sb.WriteString("?")
				values = append(values, row[j])
			} else {
				sb.WriteString("NULL")
			}
			if j < len(table.Columns)-1 {
				sb.WriteString(", ")
			}
		}
		sb.WriteString(")")
		if i < len(rows)-1 {
			sb.WriteString(",\n")
		}
	}

	insertSQL := sb.String()

	// 执行插入
	if err := t.db.Exec(insertSQL, values...).Error; err != nil {
		g.Log().Errorf(ctx, "Insert failed: %v\nSQL: %s", err, insertSQL)
		return err
	}

	return nil
}

// DropTable 删除表（用于清理）
func (t *TableImporter) DropTable(ctx context.Context, tableName string) error {
	var dropSQL string

	switch t.dbType {
	case nl2sqlCommon.DBTypePostgreSQL, "postgres", "pgsql":
		dropSQL = fmt.Sprintf("DROP TABLE IF EXISTS nl2sql.%s CASCADE", tableName)
	default:
		return fmt.Errorf("unsupported database type: %s", t.dbType)
	}

	if err := t.db.Exec(dropSQL).Error; err != nil {
		return fmt.Errorf("failed to drop table: %w", err)
	}

	g.Log().Infof(ctx, "Table dropped: %s", tableName)
	return nil
}

// TableExists 检查表是否存在
func (t *TableImporter) TableExists(ctx context.Context, tableName string) (bool, error) {
	var exists bool
	var query string

	switch t.dbType {
	case nl2sqlCommon.DBTypePostgreSQL, "postgres", "pgsql":
		query = `
			SELECT EXISTS (
				SELECT 1
				FROM information_schema.tables
				WHERE table_schema = 'nl2sql'
				AND table_name = ?
			)
		`
	default:
		return false, fmt.Errorf("unsupported database type: %s", t.dbType)
	}

	if err := t.db.Raw(query, tableName).Scan(&exists).Error; err != nil {
		return false, err
	}

	return exists, nil
}
