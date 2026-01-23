package parser

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

// FileParser CSV/Excel文件解析器
type FileParser struct {
	db *gorm.DB
}

// NewFileParser 创建文件解析器
func NewFileParser(db *gorm.DB) *FileParser {
	return &FileParser{
		db: db,
	}
}

// ParsedTable 解析后的表结构
type ParsedTable struct {
	TableName string
	Columns   []Column
	Rows      [][]interface{}
}

// Column 列定义
type Column struct {
	Name     string
	DataType string // TEXT, INTEGER, FLOAT, BOOLEAN, TIMESTAMP
	Nullable bool
}

// ParseFile 解析CSV或Excel文件
func (p *FileParser) ParseFile(ctx context.Context, filePath string) (*ParsedTable, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".csv":
		return p.parseCSV(ctx, filePath)
	case ".xlsx", ".xls":
		return p.parseExcel(ctx, filePath)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
}

// parseCSV 解析CSV文件
func (p *FileParser) parseCSV(ctx context.Context, filePath string) (*ParsedTable, error) {
	g.Log().Infof(ctx, "Parsing CSV file: %s", filePath)

	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Newf(errors.ErrFileReadFailed, "failed to open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	// 读取表头
	headers, err := reader.Read()
	if err != nil {
		return nil, errors.Newf(errors.ErrFileReadFailed, "failed to read CSV headers: %v", err)
	}

	// 清理表头，确保有效列名
	for i, header := range headers {
		headers[i] = sanitizeColumnName(header)
		if headers[i] == "" {
			headers[i] = fmt.Sprintf("column_%d", i+1)
		}
	}

	// 读取数据行用于类型推断
	var rows [][]string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			g.Log().Warningf(ctx, "Failed to read CSV row: %v", err)
			continue
		}
		rows = append(rows, row)
	}

	g.Log().Infof(ctx, "CSV file parsed: %d columns, %d rows", len(headers), len(rows))

	// 推断列类型
	columns := inferColumnTypes(headers, rows)

	// 转换数据为interface{}
	var dataRows [][]interface{}
	for _, row := range rows {
		var dataRow []interface{}
		for i, val := range row {
			if i >= len(columns) {
				break
			}
			dataRow = append(dataRow, convertValue(val, columns[i].DataType))
		}
		dataRows = append(dataRows, dataRow)
	}

	// 生成表名（从文件名）
	tableName := sanitizeTableName(filepath.Base(filePath))

	return &ParsedTable{
		TableName: tableName,
		Columns:   columns,
		Rows:      dataRows,
	}, nil
}

// parseExcel 解析Excel文件
func (p *FileParser) parseExcel(ctx context.Context, filePath string) (*ParsedTable, error) {
	g.Log().Infof(ctx, "Parsing Excel file: %s", filePath)

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, errors.Newf(errors.ErrFileReadFailed, "failed to open Excel file: %v", err)
	}
	defer f.Close()

	// 获取第一个工作表
	sheetName := f.GetSheetName(0)
	if sheetName == "" {
		return nil, errors.Newf(errors.ErrFileReadFailed, "no sheets found in Excel file")
	}

	g.Log().Infof(ctx, "Reading Excel sheet: %s", sheetName)

	// 读取所有行
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, errors.Newf(errors.ErrFileReadFailed, "failed to read Excel rows: %v", err)
	}

	if len(rows) == 0 {
		return nil, errors.Newf(errors.ErrFileReadFailed, "Excel file is empty")
	}

	// 第一行作为表头
	headers := rows[0]
	for i, header := range headers {
		headers[i] = sanitizeColumnName(header)
		if headers[i] == "" {
			headers[i] = fmt.Sprintf("column_%d", i+1)
		}
	}

	// 数据行
	dataRowsStr := rows[1:]

	g.Log().Infof(ctx, "Excel file parsed: %d columns, %d rows", len(headers), len(dataRowsStr))

	// 推断列类型
	columns := inferColumnTypes(headers, dataRowsStr)

	// 转换数据为interface{}
	var dataRows [][]interface{}
	for _, row := range dataRowsStr {
		var dataRow []interface{}
		for i, val := range row {
			if i >= len(columns) {
				break
			}
			dataRow = append(dataRow, convertValue(val, columns[i].DataType))
		}
		// 填充缺失的列
		for len(dataRow) < len(columns) {
			dataRow = append(dataRow, nil)
		}
		dataRows = append(dataRows, dataRow)
	}

	// 生成表名（从文件名）
	tableName := sanitizeTableName(filepath.Base(filePath))

	return &ParsedTable{
		TableName: tableName,
		Columns:   columns,
		Rows:      dataRows,
	}, nil
}

// inferColumnTypes 推断列类型
func inferColumnTypes(headers []string, rows [][]string) []Column {
	columns := make([]Column, len(headers))

	for i, header := range headers {
		columns[i] = Column{
			Name:     header,
			DataType: "TEXT", // 默认TEXT类型
			Nullable: true,
		}

		// 采样前100行推断类型
		sampleSize := 100
		if sampleSize > len(rows) {
			sampleSize = len(rows)
		}

		hasNull := false
		isInteger := true
		isFloat := true

		for j := 0; j < sampleSize; j++ {
			if i >= len(rows[j]) {
				hasNull = true
				continue
			}

			val := strings.TrimSpace(rows[j][i])
			if val == "" || val == "NULL" || val == "null" {
				hasNull = true
				continue
			}

			// 检查是否为整数
			if isInteger && !isIntegerValue(val) {
				isInteger = false
			}

			// 检查是否为浮点数
			if isFloat && !isFloatValue(val) {
				isFloat = false
			}
		}

		// 确定数据类型
		if isInteger {
			columns[i].DataType = "INTEGER"
		} else if isFloat {
			columns[i].DataType = "FLOAT"
		} else {
			columns[i].DataType = "TEXT"
		}

		columns[i].Nullable = hasNull
	}

	return columns
}

// isIntegerValue 判断是否为整数
func isIntegerValue(val string) bool {
	if val == "" {
		return false
	}
	for i, r := range val {
		if i == 0 && (r == '-' || r == '+') {
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// isFloatValue 判断是否为浮点数
func isFloatValue(val string) bool {
	if val == "" {
		return false
	}
	hasDot := false
	for i, r := range val {
		if i == 0 && (r == '-' || r == '+') {
			continue
		}
		if r == '.' {
			if hasDot {
				return false
			}
			hasDot = true
			continue
		}
		if r < '0' || r > '9' {
			return false
		}
	}
	return hasDot
}

// convertValue 转换值为目标类型
func convertValue(val string, dataType string) interface{} {
	val = strings.TrimSpace(val)
	if val == "" || val == "NULL" || val == "null" {
		return nil
	}

	switch dataType {
	case "INTEGER":
		var result int64
		fmt.Sscanf(val, "%d", &result)
		return result
	case "FLOAT":
		var result float64
		fmt.Sscanf(val, "%f", &result)
		return result
	default:
		return val
	}
}

// sanitizeColumnName 清理列名，确保符合数据库命名规范
func sanitizeColumnName(name string) string {
	name = strings.TrimSpace(name)
	// 移除非法字符
	var builder strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			builder.WriteRune(r)
		} else if r == ' ' || r == '-' {
			builder.WriteRune('_')
		}
	}
	result := builder.String()

	// 确保以字母或下划线开头
	if len(result) > 0 && result[0] >= '0' && result[0] <= '9' {
		result = "_" + result
	}

	// 转换为小写
	result = strings.ToLower(result)

	// 限制长度
	if utf8.RuneCountInString(result) > 63 {
		result = string([]rune(result)[:63])
	}

	return result
}

// sanitizeTableName 清理表名，从文件名生成
func sanitizeTableName(fileName string) string {
	// 移除扩展名
	name := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	sanitized := sanitizeColumnName(name)

	// 如果清理后的名称为空（例如纯中文文件名），使用默认名称
	if sanitized == "" {
		// 使用 "table_" 加上原始文件名的哈希值（保证至少8位）
		hash := hashString(name)
		// 如果哈希值不足8位，补零
		if len(hash) < 8 {
			hash = fmt.Sprintf("%08s", hash)
		}
		sanitized = fmt.Sprintf("table_%s", hash[:8])
	}

	return sanitized
}

// hashString 生成字符串的简单哈希值（用于生成表名）
func hashString(s string) string {
	h := 0
	for _, c := range s {
		h = h*31 + int(c)
	}
	// 返回至少8位的十六进制字符串
	return fmt.Sprintf("%08x", h)
}
