package schema

import (
	"context"
	"encoding/json"
	"fmt"

	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"
	"github.com/Malowking/kbgo/nl2sql/datasource"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// SchemaBuilder Schema构建器
type SchemaBuilder struct {
	db *gorm.DB
}

// NewSchemaBuilder 创建Schema构建器
func NewSchemaBuilder(db *gorm.DB) *SchemaBuilder {
	return &SchemaBuilder{
		db: db,
	}
}

// BuildFromJDBC 从JDBC数据源构建Schema
func (b *SchemaBuilder) BuildFromJDBC(ctx context.Context, datasourceID string, connector *datasource.JDBCConnector) (string, error) {
	// 1. 获取数据源并标记Schema已解析
	var ds dbgorm.NL2SQLDataSource
	if err := b.db.First(&ds, "id = ?", datasourceID).Error; err != nil {
		return "", fmt.Errorf("数据源不存在: %w", err)
	}

	// 2. 获取所有表
	tables, err := connector.GetTables(ctx)
	if err != nil {
		return "", fmt.Errorf("获取表列表失败: %w", err)
	}

	// 3. 遍历表，构建元数据
	for _, tableName := range tables {
		if err := b.buildTableMetadata(ctx, datasourceID, tableName, connector); err != nil {
			return "", fmt.Errorf("构建表 %s 元数据失败: %w", tableName, err)
		}
	}

	// 4. 推断关系（基于外键）
	if err := b.inferRelations(ctx, datasourceID); err != nil {
		return "", fmt.Errorf("推断关系失败: %w", err)
	}
	return datasourceID, nil
}

// BuildFromNL2SQLSchema 从nl2sql schema中构建元数据（用于CSV/Excel数据源）
func (b *SchemaBuilder) BuildFromNL2SQLSchema(ctx context.Context, datasourceID string) (string, error) {
	// 1. 获取数据源
	var ds dbgorm.NL2SQLDataSource
	if err := b.db.First(&ds, "id = ?", datasourceID).Error; err != nil {
		return "", fmt.Errorf("数据源不存在: %w", err)
	}

	// 2. 从 nl2sql_tables 表获取该数据源下的所有表
	var tables []dbgorm.NL2SQLTable
	if err := b.db.Where("datasource_id = ?", datasourceID).Find(&tables).Error; err != nil {
		return "", fmt.Errorf("获取表列表失败: %w", err)
	}

	if len(tables) == 0 {
		return "", fmt.Errorf("数据源下没有找到任何表")
	}

	// 3. 遍历所有表，构建元数据
	for _, table := range tables {
		if err := b.BuildTableFromNL2SQLSchema(ctx, datasourceID, table.Name, table.DisplayName, table.FilePath); err != nil {
			return "", fmt.Errorf("构建表 %s 元数据失败: %w", table.Name, err)
		}
	}

	// 4. 推断关系
	if err := b.inferRelations(ctx, datasourceID); err != nil {
		g.Log().Warningf(ctx, "推断关系失败（非致命错误）: %v", err)
		// 不返回错误，关系推断失败不影响主流程
	}

	return datasourceID, nil
}

// BuildTableFromNL2SQLSchema 从nl2sql schema中构建单个表的元数据
func (b *SchemaBuilder) BuildTableFromNL2SQLSchema(ctx context.Context, datasourceID, tableName, displayName, filePath string) error {
	// 1. 检查表是否已存在
	var existingTable dbgorm.NL2SQLTable
	err := b.db.Where("datasource_id = ? AND table_name = ?", datasourceID, tableName).First(&existingTable).Error
	if err == nil {
		g.Log().Infof(ctx, "Table %s already exists for datasource %s, skipping", tableName, datasourceID)
		return nil
	} else if err != gorm.ErrRecordNotFound {
		// 不是 "记录不存在" 的错误，说明是真正的数据库错误
		return fmt.Errorf("%w", err)
	}
	// err == gorm.ErrRecordNotFound，表不存在，继续创建

	// 2. 从nl2sql schema查询表结构
	query := `
		SELECT column_name, data_type, is_nullable
		FROM information_schema.columns
		WHERE table_schema = 'nl2sql' AND table_name = $1
		ORDER BY ordinal_position
	`

	type ColumnInfo struct {
		ColumnName string `gorm:"column:column_name"`
		DataType   string `gorm:"column:data_type"`
		IsNullable string `gorm:"column:is_nullable"`
	}

	var columns []ColumnInfo
	if err := b.db.Raw(query, tableName).Scan(&columns).Error; err != nil {
		return fmt.Errorf("查询表结构失败: %w", err)
	}

	if len(columns) == 0 {
		return fmt.Errorf("表 %s 不存在于nl2sql schema中", tableName)
	}

	// 3. 获取表行数
	var rowCount int64
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM nl2sql.%s", tableName)
	if err := b.db.Raw(countQuery).Scan(&rowCount).Error; err != nil {
		rowCount = 0
	}

	table := &dbgorm.NL2SQLTable{
		DatasourceID:     datasourceID,
		Name:             tableName,
		DisplayName:      displayName,
		Description:      "", // 后续由LLM填充
		RowCountEstimate: rowCount,
		PrimaryKey:       "id", // nl2sql表默认使用id作为主键
		TimeColumn:       "",
		FilePath:         filePath, // 保存文件路径
		UsagePatterns:    datatypes.JSON([]byte("[]")),
	}

	if err := b.db.Create(table).Error; err != nil {
		return fmt.Errorf("创建Table记录失败: %w", err)
	}

	// 5. 创建Column记录（跳过id列，这是自动生成的）
	for _, col := range columns {
		if col.ColumnName == "id" {
			continue
		}

		// 采样数据
		sampleQuery := fmt.Sprintf("SELECT %s FROM nl2sql.%s LIMIT 5", col.ColumnName, tableName)
		var sampleValues []interface{}
		rows, err := b.db.Raw(sampleQuery).Rows()
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var val interface{}
				if err := rows.Scan(&val); err == nil && val != nil {
					sampleValues = append(sampleValues, val)
				}
			}
		}

		examples := make([]string, 0)
		for _, val := range sampleValues {
			examples = append(examples, fmt.Sprintf("%v", val))
		}
		examplesJSON, _ := json.Marshal(examples)

		column := &dbgorm.NL2SQLColumn{
			TableID:       table.ID,
			ColumnName:    col.ColumnName,
			DisplayName:   displayName,
			DataType:      col.DataType,
			Nullable:      col.IsNullable == "YES",
			Description:   "", // 后续由LLM填充
			SemanticType:  inferSemanticType(col.ColumnName, col.DataType),
			Examples:      datatypes.JSON(examplesJSON),
			Enums:         datatypes.JSON([]byte("[]")),
			CommonFilters: datatypes.JSON([]byte("[]")),
		}

		if err := b.db.Create(column).Error; err != nil {
			return fmt.Errorf("创建Column记录失败: %w", err)
		}
	}

	g.Log().Infof(ctx, "Built metadata for table %s in datasource %s", tableName, datasourceID)
	return nil
}

// buildTableMetadata 构建单个表的元数据
func (b *SchemaBuilder) buildTableMetadata(ctx context.Context, datasourceID string, tableName string, connector *datasource.JDBCConnector) error {
	// 1. 获取表结构
	tableSchema, err := connector.GetTableSchema(ctx, tableName)
	if err != nil {
		return err
	}

	// 2. 创建Table记录
	table := &dbgorm.NL2SQLTable{
		DatasourceID:     datasourceID,
		Name:             tableName,
		DisplayName:      formatDisplayName(tableName),
		Description:      "", // 后续由LLM填充
		RowCountEstimate: tableSchema.RowCount,
		PrimaryKey:       joinStrings(tableSchema.PrimaryKeys, ","),
		TimeColumn:       inferTimeColumn(tableSchema.Columns),
		UsagePatterns:    datatypes.JSON([]byte("[]")),
	}

	if err := b.db.Create(table).Error; err != nil {
		return fmt.Errorf("创建Table记录失败: %w", err)
	}

	// 3. 创建Column记录
	for _, col := range tableSchema.Columns {
		// 采样数据
		sampleData, _ := connector.SampleRows(ctx, tableName, 5)
		examples := extractColumnExamples(sampleData, col.Name, 5)

		examplesJSON, _ := json.Marshal(examples)

		column := &dbgorm.NL2SQLColumn{
			TableID:       table.ID,
			ColumnName:    col.Name,
			DisplayName:   formatDisplayName(col.Name),
			DataType:      col.DataType,
			Nullable:      col.Nullable,
			Description:   "", // 后续由LLM填充
			SemanticType:  inferSemanticType(col.Name, col.DataType),
			Examples:      datatypes.JSON(examplesJSON),
			Enums:         datatypes.JSON([]byte("[]")),
			CommonFilters: datatypes.JSON([]byte("[]")),
		}

		if err := b.db.Create(column).Error; err != nil {
			return fmt.Errorf("创建Column记录失败: %w", err)
		}
	}

	return nil
}

// inferRelations 推断表之间的关系
func (b *SchemaBuilder) inferRelations(ctx context.Context, datasourceID string) error {
	// 简化实现：基于命名约定推断关系
	// 例如：order_items.order_id -> orders.id

	var tables []dbgorm.NL2SQLTable
	if err := b.db.Where("datasource_id = ?", datasourceID).Find(&tables).Error; err != nil {
		return err
	}

	// 为每个表查找外键候选
	for _, table := range tables {
		var columns []dbgorm.NL2SQLColumn
		if err := b.db.Where("table_id = ?", table.ID).Find(&columns).Error; err != nil {
			return err
		}

		for _, col := range columns {
			// 检查是否可能是外键（命名以_id结尾）
			if !isLikelyForeignKey(col.ColumnName) {
				continue
			}

			// 尝试匹配目标表
			targetTableName := extractTableNameFromFK(col.ColumnName)
			var targetTable dbgorm.NL2SQLTable
			if err := b.db.Where("datasource_id = ? AND table_name = ?", datasourceID, targetTableName).First(&targetTable).Error; err != nil {
				continue // 找不到目标表，跳过
			}

			// 查找目标表的主键列
			var targetPKColumns []dbgorm.NL2SQLColumn
			if err := b.db.Where("table_id = ? AND column_name IN (?)", targetTable.ID, []string{"id", targetTable.PrimaryKey}).Find(&targetPKColumns).Error; err != nil {
				continue
			}

			if len(targetPKColumns) == 0 {
				continue
			}

			// 创建关系记录
			relation := &dbgorm.NL2SQLRelation{
				DatasourceID: datasourceID,
				RelationID:   fmt.Sprintf("rel_%s_%s", table.Name, col.ColumnName),
				FromTableID:  table.ID,
				FromColumn:   col.ColumnName,
				ToTableID:    targetTable.ID,
				ToColumn:     targetPKColumns[0].ColumnName,
				RelationType: nl2sqlCommon.RelationManyToOne,
				JoinType:     "INNER",
				Description:  fmt.Sprintf("%s.%s -> %s.%s", table.Name, col.ColumnName, targetTable.Name, targetPKColumns[0].ColumnName),
			}

			_ = b.db.Create(relation) // 忽略重复错误
		}
	}

	return nil
}

// EnrichWithLLM 使用LLM增强Schema描述
func (b *SchemaBuilder) EnrichWithLLM(ctx context.Context, datasourceID string, llmFunc func(prompt string) (string, error)) error {
	// 1. 获取所有表
	var tables []dbgorm.NL2SQLTable
	if err := b.db.Where("datasource_id = ?", datasourceID).Find(&tables).Error; err != nil {
		return err
	}

	// 2. 为每个表生成描述
	for _, table := range tables {
		// 获取该表的列信息
		var columns []dbgorm.NL2SQLColumn
		if err := b.db.Where("table_id = ?", table.ID).Find(&columns).Error; err != nil {
			return err
		}

		// 构造Prompt
		prompt := buildTableDescriptionPrompt(table.Name, columns)

		// 调用LLM
		description, err := llmFunc(prompt)
		if err != nil {
			return fmt.Errorf("LLM生成表描述失败: %w", err)
		}

		// 更新表描述
		if err := b.db.Model(&table).Update("description", description).Error; err != nil {
			return err
		}

		// 3. 为每个列生成描述和语义类型
		for _, column := range columns {
			colPrompt := buildColumnDescriptionPrompt(table.Name, column.ColumnName, column.DataType, string(column.Examples))
			colDescription, err := llmFunc(colPrompt)
			if err != nil {
				continue // 忽略单个列的错误
			}

			if err := b.db.Model(&column).Update("description", colDescription).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

// buildTableDescriptionPrompt 构建表描述Prompt
func buildTableDescriptionPrompt(tableName string, columns []dbgorm.NL2SQLColumn) string {
	columnsDesc := ""
	for _, col := range columns {
		columnsDesc += fmt.Sprintf("- %s (%s)\n", col.ColumnName, col.DataType)
	}

	return fmt.Sprintf(`
请为以下数据库表生成简洁的业务描述（50字以内）：

表名：%s
列信息：
%s

请只返回描述文本，不要包含其他内容。
`, tableName, columnsDesc)
}

// buildColumnDescriptionPrompt 构建列描述Prompt
func buildColumnDescriptionPrompt(tableName, columnName, dataType, examples string) string {
	return fmt.Sprintf(`
请为以下数据库列生成简洁的业务描述（30字以内）：

表名：%s
列名：%s
数据类型：%s
示例值：%s

请只返回描述文本，不要包含其他内容。
`, tableName, columnName, dataType, examples)
}

// 辅助函数

func formatDisplayName(name string) string {
	// 简单实现：去除下划线并转为Title Case
	return name
}

func joinStrings(arr []string, sep string) string {
	result := ""
	for i, s := range arr {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

func inferTimeColumn(columns []datasource.ColumnSchema) string {
	for _, col := range columns {
		if isTimeType(col.DataType) && (col.Name == "created_at" || col.Name == "updated_at" || col.Name == "time") {
			return col.Name
		}
	}
	return ""
}

func isTimeType(dataType string) bool {
	return dataType == "timestamp" || dataType == "datetime" || dataType == "date"
}

func inferSemanticType(columnName, dataType string) string {
	if columnName == "id" || columnName == "ID" {
		return "id"
	}
	if isTimeType(dataType) {
		return "time"
	}
	if dataType == "decimal" || dataType == "numeric" || dataType == "money" {
		return "currency"
	}
	return "text"
}

func extractColumnExamples(rows []map[string]interface{}, columnName string, limit int) []string {
	examples := make([]string, 0)
	for _, row := range rows {
		if val, ok := row[columnName]; ok && val != nil {
			examples = append(examples, fmt.Sprintf("%v", val))
			if len(examples) >= limit {
				break
			}
		}
	}
	return examples
}

func isLikelyForeignKey(columnName string) bool {
	return len(columnName) > 3 && columnName[len(columnName)-3:] == "_id"
}

func extractTableNameFromFK(fkColumnName string) string {
	// order_id -> orders
	name := fkColumnName[:len(fkColumnName)-3]
	return name + "s" // 简化：假设复数形式
}
