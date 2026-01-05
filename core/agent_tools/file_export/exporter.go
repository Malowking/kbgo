package file_export

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// ExportFormat 导出文件格式
type ExportFormat string

const (
	FormatCSV      ExportFormat = "csv"
	FormatExcel    ExportFormat = "xlsx"
	FormatMarkdown ExportFormat = "md"
	FormatText     ExportFormat = "txt"
	FormatJSON     ExportFormat = "json"
)

// ExportRequest 导出请求
type ExportRequest struct {
	Format      ExportFormat             // 导出格式
	Filename    string                   // 文件名（不含扩展名）
	Columns     []string                 // 列名
	Data        []map[string]interface{} // 数据
	Title       string                   // 文件标题（可选，用于Excel等）
	Description string                   // 描述信息（可选）
}

// ExportResult 导出结果
type ExportResult struct {
	FilePath    string    // 生成的文件路径
	FileURL     string    // 文件下载URL（相对路径）
	Filename    string    // 文件名（含扩展名）
	Format      string    // 文件格式
	Size        int64     // 文件大小（字节）
	RowCount    int       // 导出的行数
	GeneratedAt time.Time // 生成时间
}

// FileExporter 文件导出器
type FileExporter struct {
	baseDir string // 基础导出目录，复用upload目录结构
}

// NewFileExporter 创建文件导出器
func NewFileExporter(baseDir string) *FileExporter {
	if baseDir == "" {
		baseDir = "upload" // 复用upload目录
	}
	return &FileExporter{
		baseDir: baseDir,
	}
}

// Export 导出数据到文件
func (e *FileExporter) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	// 验证请求
	if err := e.validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid export request: %w", err)
	}

	// 构建目标目录（使用 upload/export 子目录）
	targetDir := filepath.Join(e.baseDir, "export")

	// 确保目录存在
	if !gfile.Exists(targetDir) {
		if err := gfile.Mkdir(targetDir); err != nil {
			return nil, fmt.Errorf("failed to create directory %s: %v", targetDir, err)
		}
	}

	// 生成UUID文件名
	fileExt := string(req.Format)
	uuidFileName := strings.ReplaceAll(uuid.New().String(), "-", "") + "." + fileExt
	targetPath := filepath.Join(targetDir, uuidFileName)

	g.Log().Infof(ctx, "Exporting data to file: %s", targetPath)

	// 根据格式导出
	var err error
	var size int64

	switch req.Format {
	case FormatCSV:
		size, err = e.exportCSV(targetPath, req)
	case FormatExcel:
		size, err = e.exportExcel(targetPath, req)
	case FormatJSON:
		size, err = e.exportJSON(targetPath, req)
	case FormatMarkdown:
		size, err = e.exportMarkdown(targetPath, req)
	case FormatText:
		size, err = e.exportText(targetPath, req)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", req.Format)
	}

	if err != nil {
		return nil, fmt.Errorf("export failed: %w", err)
	}

	// 构建结果
	result := &ExportResult{
		FilePath:    targetPath,
		FileURL:     filepath.Join("/", "upload", "export", uuidFileName), // 文件访问URL
		Filename:    req.Filename + "." + fileExt,
		Format:      string(req.Format),
		Size:        size,
		RowCount:    len(req.Data),
		GeneratedAt: time.Now(),
	}

	g.Log().Infof(ctx, "Export completed: %s, size: %d bytes, rows: %d", result.Filename, size, result.RowCount)

	return result, nil
}

// exportCSV 导出为CSV格式
func (e *FileExporter) exportCSV(filePath string, req *ExportRequest) (int64, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create CSV file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入UTF-8 BOM（Excel兼容）
	file.Write([]byte{0xEF, 0xBB, 0xBF})

	// 写入表头
	if err := writer.Write(req.Columns); err != nil {
		return 0, fmt.Errorf("failed to write CSV header: %w", err)
	}

	// 写入数据
	for _, row := range req.Data {
		record := make([]string, len(req.Columns))
		for i, col := range req.Columns {
			record[i] = fmt.Sprintf("%v", row[col])
		}
		if err := writer.Write(record); err != nil {
			return 0, fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	// 获取文件大小
	info, _ := file.Stat()
	return info.Size(), nil
}

// exportExcel 导出为Excel格式
func (e *FileExporter) exportExcel(filePath string, req *ExportRequest) (int64, error) {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		return 0, fmt.Errorf("failed to create sheet: %w", err)
	}

	// 设置标题（如果有）
	rowIndex := 1
	if req.Title != "" {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowIndex), req.Title)
		// 设置标题样式
		titleStyle, _ := f.NewStyle(&excelize.Style{
			Font: &excelize.Font{Bold: true, Size: 14},
		})
		f.SetCellStyle(sheetName, fmt.Sprintf("A%d", rowIndex), fmt.Sprintf("A%d", rowIndex), titleStyle)
		rowIndex++
	}

	// 写入表头
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
	})

	for i, col := range req.Columns {
		cell := fmt.Sprintf("%c%d", 'A'+i, rowIndex)
		f.SetCellValue(sheetName, cell, col)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}
	rowIndex++

	// 写入数据
	for _, row := range req.Data {
		for i, col := range req.Columns {
			cell := fmt.Sprintf("%c%d", 'A'+i, rowIndex)
			f.SetCellValue(sheetName, cell, row[col])
		}
		rowIndex++
	}

	// 自动调整列宽
	for i := range req.Columns {
		colName := string('A' + i)
		f.SetColWidth(sheetName, colName, colName, 15)
	}

	f.SetActiveSheet(index)

	// 保存文件
	if err := f.SaveAs(filePath); err != nil {
		return 0, fmt.Errorf("failed to save Excel file: %w", err)
	}

	// 获取文件大小
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// exportJSON 导出为JSON格式
func (e *FileExporter) exportJSON(filePath string, req *ExportRequest) (int64, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create JSON file: %w", err)
	}
	defer file.Close()

	// 构建导出对象
	exportData := map[string]interface{}{
		"columns": req.Columns,
		"data":    req.Data,
		"count":   len(req.Data),
	}

	if req.Title != "" {
		exportData["title"] = req.Title
	}
	if req.Description != "" {
		exportData["description"] = req.Description
	}

	// 格式化输出
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(exportData); err != nil {
		return 0, fmt.Errorf("failed to write JSON: %w", err)
	}

	// 获取文件大小
	info, _ := file.Stat()
	return info.Size(), nil
}

// exportMarkdown 导出为Markdown表格格式
func (e *FileExporter) exportMarkdown(filePath string, req *ExportRequest) (int64, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create Markdown file: %w", err)
	}
	defer file.Close()

	// 写入标题
	if req.Title != "" {
		file.WriteString(fmt.Sprintf("# %s\n\n", req.Title))
	}

	// 写入描述
	if req.Description != "" {
		file.WriteString(fmt.Sprintf("%s\n\n", req.Description))
	}

	// 写入表头
	file.WriteString("|")
	for _, col := range req.Columns {
		file.WriteString(fmt.Sprintf(" %s |", col))
	}
	file.WriteString("\n")

	// 写入分隔符
	file.WriteString("|")
	for range req.Columns {
		file.WriteString(" --- |")
	}
	file.WriteString("\n")

	// 写入数据
	for _, row := range req.Data {
		file.WriteString("|")
		for _, col := range req.Columns {
			file.WriteString(fmt.Sprintf(" %v |", row[col]))
		}
		file.WriteString("\n")
	}

	// 写入统计信息
	file.WriteString(fmt.Sprintf("\n总计: %d 行\n", len(req.Data)))

	// 获取文件大小
	info, _ := file.Stat()
	return info.Size(), nil
}

// exportText 导出为纯文本格式
func (e *FileExporter) exportText(filePath string, req *ExportRequest) (int64, error) {
	file, err := os.Create(filePath)
	if err != nil {
		return 0, fmt.Errorf("failed to create text file: %w", err)
	}
	defer file.Close()

	// 写入标题
	if req.Title != "" {
		file.WriteString(req.Title + "\n")
		file.WriteString(strings.Repeat("=", len(req.Title)) + "\n\n")
	}

	// 写入描述
	if req.Description != "" {
		file.WriteString(req.Description + "\n\n")
	}

	// 计算每列的最大宽度
	colWidths := make([]int, len(req.Columns))
	for i, col := range req.Columns {
		colWidths[i] = len(col)
	}
	for _, row := range req.Data {
		for i, col := range req.Columns {
			valueLen := len(fmt.Sprintf("%v", row[col]))
			if valueLen > colWidths[i] {
				colWidths[i] = valueLen
			}
		}
	}

	// 写入表头
	for i, col := range req.Columns {
		file.WriteString(fmt.Sprintf("%-*s  ", colWidths[i], col))
	}
	file.WriteString("\n")

	// 写入分隔线
	for _, width := range colWidths {
		file.WriteString(strings.Repeat("-", width) + "  ")
	}
	file.WriteString("\n")

	// 写入数据
	for _, row := range req.Data {
		for i, col := range req.Columns {
			file.WriteString(fmt.Sprintf("%-*v  ", colWidths[i], row[col]))
		}
		file.WriteString("\n")
	}

	// 写入统计信息
	file.WriteString(fmt.Sprintf("\n总计: %d 行\n", len(req.Data)))

	// 获取文件大小
	info, _ := file.Stat()
	return info.Size(), nil
}

// validateRequest 验证导出请求
func (e *FileExporter) validateRequest(req *ExportRequest) error {
	if req.Filename == "" {
		return fmt.Errorf("filename is required")
	}

	if len(req.Columns) == 0 {
		return fmt.Errorf("columns are required")
	}

	if len(req.Data) == 0 {
		return fmt.Errorf("no data to export")
	}

	return nil
}

// GetSupportedFormats 获取支持的格式列表
func (e *FileExporter) GetSupportedFormats() []ExportFormat {
	return []ExportFormat{
		FormatCSV,
		FormatExcel,
		FormatJSON,
		FormatMarkdown,
		FormatText,
	}
}
