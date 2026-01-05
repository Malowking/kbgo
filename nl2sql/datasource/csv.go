package datasource

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"
)

// CSVDataSource CSV数据源
type CSVDataSource struct {
	config   *Config
	filePath string
	headers  []string
	data     [][]string
}

// NewCSVDataSource 创建CSV数据源
func NewCSVDataSource(config *Config) (*CSVDataSource, error) {
	filePath, ok := config.Settings["file_path"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少file_path配置")
	}

	return &CSVDataSource{
		config:   config,
		filePath: filePath,
	}, nil
}

// Connect 连接（加载CSV文件）
func (ds *CSVDataSource) Connect(ctx context.Context) error {
	file, err := os.Open(ds.filePath)
	if err != nil {
		return fmt.Errorf("打开CSV文件失败: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return fmt.Errorf("读取CSV文件失败: %w", err)
	}

	if len(records) == 0 {
		return fmt.Errorf("CSV文件为空")
	}

	ds.headers = records[0]
	ds.data = records[1:]

	return nil
}

// Close 关闭
func (ds *CSVDataSource) Close() error {
	// CSV无需关闭
	return nil
}

// GetTables 获取表名（CSV视为单表）
func (ds *CSVDataSource) GetTables(ctx context.Context) ([]string, error) {
	// 从文件名提取表名
	fileName := ds.filePath
	if idx := strings.LastIndex(fileName, "/"); idx != -1 {
		fileName = fileName[idx+1:]
	}
	if idx := strings.LastIndex(fileName, "."); idx != -1 {
		fileName = fileName[:idx]
	}

	return []string{fileName}, nil
}

// GetTableSchema 获取表结构
func (ds *CSVDataSource) GetTableSchema(ctx context.Context, tableName string) (*TableSchema, error) {
	if ds.headers == nil {
		if err := ds.Connect(ctx); err != nil {
			return nil, err
		}
	}

	schema := &TableSchema{
		TableName: tableName,
		Columns:   make([]ColumnSchema, 0, len(ds.headers)),
		RowCount:  int64(len(ds.data)),
	}

	// 推断列类型（简化版）
	for _, header := range ds.headers {
		col := ColumnSchema{
			Name:     header,
			DataType: "text", // CSV默认都是文本
			Nullable: true,
		}
		schema.Columns = append(schema.Columns, col)
	}

	return schema, nil
}

// ExecuteQuery 执行查询（简化版，仅支持SELECT *）
func (ds *CSVDataSource) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	if ds.data == nil {
		if err := ds.Connect(ctx); err != nil {
			return &QueryResult{Error: err}, err
		}
	}

	// 转换为interface{}类型
	rows := make([][]interface{}, len(ds.data))
	for i, row := range ds.data {
		rows[i] = make([]interface{}, len(row))
		for j, val := range row {
			rows[i][j] = val
		}
	}

	return &QueryResult{
		Columns: ds.headers,
		Rows:    rows,
	}, nil
}

// TestConnection 测试连接
func (ds *CSVDataSource) TestConnection(ctx context.Context) error {
	return ds.Connect(ctx)
}

// ExcelDataSource Excel数据源
type ExcelDataSource struct {
	config   *Config
	filePath string
	sheets   map[string][][]string // sheet_name -> rows data
}

// NewExcelDataSource 创建Excel数据源
func NewExcelDataSource(config *Config) (*ExcelDataSource, error) {
	filePath, ok := config.Settings["file_path"].(string)
	if !ok {
		return nil, fmt.Errorf("缺少file_path配置")
	}

	return &ExcelDataSource{
		config:   config,
		filePath: filePath,
		sheets:   make(map[string][][]string),
	}, nil
}

// Connect 连接（加载Excel文件）
func (ds *ExcelDataSource) Connect(ctx context.Context) error {
	// 注意：这里需要使用Excel库如github.com/xuri/excelize/v2
	// 由于可能没有安装该库，这里提供一个简化的实现框架
	// 实际使用时需要: go get github.com/xuri/excelize/v2

	// 检查文件是否存在
	if _, err := os.Stat(ds.filePath); os.IsNotExist(err) {
		return fmt.Errorf("Excel文件不存在: %s", ds.filePath)
	}

	// 实际实现应该使用excelize读取Excel
	// 这里提供一个占位实现，提示用户需要安装库
	return fmt.Errorf("Excel数据源需要安装excelize库: go get github.com/xuri/excelize/v2")

	/* 完整实现示例（需要安装excelize）:

	import "github.com/xuri/excelize/v2"

	f, err := excelize.OpenFile(ds.filePath)
	if err != nil {
		return fmt.Errorf("打开Excel文件失败: %w", err)
	}
	defer f.Close()

	// 读取所有sheet
	sheetList := f.GetSheetList()
	for _, sheetName := range sheetList {
		rows, err := f.GetRows(sheetName)
		if err != nil {
			continue
		}
		ds.sheets[sheetName] = rows
	}

	if len(ds.sheets) == 0 {
		return fmt.Errorf("Excel文件中没有可用的sheet")
	}

	return nil
	*/
}

// Close 关闭
func (ds *ExcelDataSource) Close() error {
	ds.sheets = nil
	return nil
}

// GetTables 获取表名（每个sheet视为一个表）
func (ds *ExcelDataSource) GetTables(ctx context.Context) ([]string, error) {
	if ds.sheets == nil || len(ds.sheets) == 0 {
		if err := ds.Connect(ctx); err != nil {
			return nil, err
		}
	}

	tables := make([]string, 0, len(ds.sheets))
	for sheetName := range ds.sheets {
		tables = append(tables, sheetName)
	}

	return tables, nil
}

// GetTableSchema 获取表结构
func (ds *ExcelDataSource) GetTableSchema(ctx context.Context, tableName string) (*TableSchema, error) {
	if ds.sheets == nil {
		if err := ds.Connect(ctx); err != nil {
			return nil, err
		}
	}

	rows, ok := ds.sheets[tableName]
	if !ok {
		return nil, fmt.Errorf("Sheet不存在: %s", tableName)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("Sheet为空: %s", tableName)
	}

	// 第一行作为表头
	headers := rows[0]
	dataRows := rows[1:]

	schema := &TableSchema{
		TableName: tableName,
		Columns:   make([]ColumnSchema, 0, len(headers)),
		RowCount:  int64(len(dataRows)),
	}

	// 推断列类型（简化版）
	for _, header := range headers {
		col := ColumnSchema{
			Name:     header,
			DataType: "text", // Excel默认都是文本，可以根据实际数据推断
			Nullable: true,
		}
		schema.Columns = append(schema.Columns, col)
	}

	return schema, nil
}

// ExecuteQuery 执行查询（简化版，仅支持SELECT *）
func (ds *ExcelDataSource) ExecuteQuery(ctx context.Context, query string) (*QueryResult, error) {
	if ds.sheets == nil {
		if err := ds.Connect(ctx); err != nil {
			return &QueryResult{Error: err}, err
		}
	}

	// 简化实现：只支持第一个sheet的全表查询
	// 实际应该解析SQL，根据表名查找对应的sheet
	var firstSheet string
	for sheetName := range ds.sheets {
		firstSheet = sheetName
		break
	}

	if firstSheet == "" {
		return &QueryResult{Error: fmt.Errorf("没有可用的sheet")}, fmt.Errorf("没有可用的sheet")
	}

	rows := ds.sheets[firstSheet]
	if len(rows) == 0 {
		return &QueryResult{Columns: []string{}, Rows: [][]interface{}{}}, nil
	}

	headers := rows[0]
	dataRows := rows[1:]

	// 转换为interface{}类型
	result := make([][]interface{}, len(dataRows))
	for i, row := range dataRows {
		result[i] = make([]interface{}, len(row))
		for j, val := range row {
			result[i][j] = val
		}
	}

	return &QueryResult{
		Columns: headers,
		Rows:    result,
	}, nil
}

// TestConnection 测试连接
func (ds *ExcelDataSource) TestConnection(ctx context.Context) error {
	return ds.Connect(ctx)
}
