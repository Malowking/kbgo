package file_export

import (
	"context"
	"os"
	"testing"

	"github.com/xuri/excelize/v2"
)

// TestColumnIndexToName 测试列索引转换函数
func TestColumnIndexToName(t *testing.T) {
	tests := []struct {
		index    int
		expected string
	}{
		{0, "A"},
		{1, "B"},
		{25, "Z"},
		{26, "AA"},
		{27, "AB"},
		{51, "AZ"},
		{52, "BA"},
		{701, "ZZ"},
		{702, "AAA"},
	}

	for _, tt := range tests {
		result := columnIndexToName(tt.index)
		if result != tt.expected {
			t.Errorf("columnIndexToName(%d) = %s; want %s", tt.index, result, tt.expected)
		}
	}
}

// TestExportExcelWithManyColumns 测试导出超过26列的Excel文件
func TestExportExcelWithManyColumns(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	exporter := NewFileExporter(tmpDir)

	// 创建30列的测试数据
	columns := make([]string, 30)
	for i := 0; i < 30; i++ {
		columns[i] = columnIndexToName(i)
	}

	// 创建测试数据
	data := []map[string]interface{}{
		{},
		{},
	}
	for i := 0; i < 30; i++ {
		data[0][columns[i]] = i
		data[1][columns[i]] = i * 2
	}

	// 导出请求
	req := &ExportRequest{
		Format:   FormatExcel,
		Filename: "test_many_columns",
		Columns:  columns,
		Data:     data,
		Title:    "Test with 30 columns",
	}

	// 执行导出
	result, err := exporter.Export(context.Background(), req)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Fatalf("Export file does not exist: %s", result.FilePath)
	}

	// 验证文件可以被Excel库打开
	f, err := excelize.OpenFile(result.FilePath)
	if err != nil {
		t.Fatalf("Failed to open exported Excel file: %v", err)
	}
	defer f.Close()

	// 验证数据
	sheetName := "Sheet1"
	rows, err := f.GetRows(sheetName)
	if err != nil {
		t.Fatalf("Failed to get rows: %v", err)
	}

	// 应该有标题行 + 表头行 + 2行数据 = 4行
	if len(rows) != 4 {
		t.Errorf("Expected 4 rows, got %d", len(rows))
	}

	// 验证表头行（第2行）有30列
	if len(rows[1]) != 30 {
		t.Errorf("Expected 30 columns in header, got %d", len(rows[1]))
	}

	// 验证列名
	for i := 0; i < 30; i++ {
		if rows[1][i] != columns[i] {
			t.Errorf("Column %d: expected %s, got %s", i, columns[i], rows[1][i])
		}
	}

	t.Logf("Successfully exported and validated Excel file with 30 columns: %s", result.FilePath)
}

// TestExportExcelBasic 测试基本的Excel导出功能
func TestExportExcelBasic(t *testing.T) {
	tmpDir := t.TempDir()
	exporter := NewFileExporter(tmpDir)

	req := &ExportRequest{
		Format:   FormatExcel,
		Filename: "test_basic",
		Columns:  []string{"Name", "Age", "City"},
		Data: []map[string]interface{}{
			{"Name": "Alice", "Age": 30, "City": "Beijing"},
			{"Name": "Bob", "Age": 25, "City": "Shanghai"},
		},
	}

	result, err := exporter.Export(context.Background(), req)
	if err != nil {
		t.Fatalf("Export failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Fatalf("Export file does not exist: %s", result.FilePath)
	}

	// 验证文件可以被Excel库打开
	f, err := excelize.OpenFile(result.FilePath)
	if err != nil {
		t.Fatalf("Failed to open exported Excel file: %v", err)
	}
	defer f.Close()

	t.Logf("Successfully exported basic Excel file: %s", result.FilePath)
}

// TestExportPDF 测试PDF导出功能
func TestExportPDF(t *testing.T) {
	tmpDir := t.TempDir()
	exporter := NewFileExporter(tmpDir)

	req := &ExportRequest{
		Format:   FormatPDF,
		Filename: "test_pdf",
		Columns:  []string{"Name", "Age", "City"},
		Data: []map[string]interface{}{
			{"Name": "Alice", "Age": 30, "City": "Beijing"},
			{"Name": "Bob", "Age": 25, "City": "Shanghai"},
			{"Name": "Charlie", "Age": 35, "City": "Guangzhou"},
		},
		Title:       "Test PDF Export",
		Description: "This is a test PDF export",
	}

	result, err := exporter.Export(context.Background(), req)
	if err != nil {
		t.Fatalf("PDF export failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Fatalf("PDF file does not exist: %s", result.FilePath)
	}

	// 验证文件大小
	if result.Size == 0 {
		t.Errorf("PDF file size is 0")
	}

	t.Logf("Successfully exported PDF file: %s (size: %d bytes)", result.FilePath, result.Size)
}

// TestExportDOCX 测试DOCX导出功能
func TestExportDOCX(t *testing.T) {
	tmpDir := t.TempDir()
	exporter := NewFileExporter(tmpDir)

	req := &ExportRequest{
		Format:   FormatDOCX,
		Filename: "test_docx",
		Columns:  []string{"Name", "Age", "City"},
		Data: []map[string]interface{}{
			{"Name": "Alice", "Age": 30, "City": "Beijing"},
			{"Name": "Bob", "Age": 25, "City": "Shanghai"},
			{"Name": "Charlie", "Age": 35, "City": "Guangzhou"},
		},
		Title:       "Test DOCX Export",
		Description: "This is a test DOCX export",
	}

	result, err := exporter.Export(context.Background(), req)
	if err != nil {
		t.Fatalf("DOCX export failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Fatalf("DOCX file does not exist: %s", result.FilePath)
	}

	// 验证文件大小
	if result.Size == 0 {
		t.Errorf("DOCX file size is 0")
	}

	t.Logf("Successfully exported DOCX file: %s (size: %d bytes)", result.FilePath, result.Size)
}

// TestGetSupportedFormats 测试获取支持的格式列表
func TestGetSupportedFormats(t *testing.T) {
	exporter := NewFileExporter("")
	formats := exporter.GetSupportedFormats()

	expectedFormats := []ExportFormat{
		FormatCSV,
		FormatExcel,
		FormatJSON,
		FormatMarkdown,
		FormatText,
		FormatPDF,
		FormatDOCX,
	}

	if len(formats) != len(expectedFormats) {
		t.Errorf("Expected %d formats, got %d", len(expectedFormats), len(formats))
	}

	// 验证所有格式都存在
	formatMap := make(map[ExportFormat]bool)
	for _, format := range formats {
		formatMap[format] = true
	}

	for _, expected := range expectedFormats {
		if !formatMap[expected] {
			t.Errorf("Expected format %s not found in supported formats", expected)
		}
	}

	t.Logf("Supported formats: %v", formats)
}
