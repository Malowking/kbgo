package kbgo

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/core/file_store"
	"github.com/Malowking/kbgo/internal/dao"
	nl2sqlCommon "github.com/Malowking/kbgo/nl2sql/common"
	"github.com/Malowking/kbgo/nl2sql/parser"
	"github.com/Malowking/kbgo/nl2sql/schema"
	"github.com/Malowking/kbgo/nl2sql/service"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
)

// NL2SQLUploadFile 上传CSV/Excel文件并创建数据源
func (c *ControllerV1) NL2SQLUploadFile(ctx context.Context, req *v1.NL2SQLUploadFileReq) (res *v1.NL2SQLUploadFileRes, err error) {
	g.Log().Infof(ctx, "NL2SQLUploadFile request - Name: %s, FileName: %s", req.Name, req.File.Filename)

	// 1. 验证文件类型
	ext := strings.ToLower(filepath.Ext(req.File.Filename))
	if ext != ".csv" && ext != ".xlsx" && ext != ".xls" {
		return nil, fmt.Errorf("不支持的文件类型，仅支持 CSV 和 Excel 文件")
	}

	// 确定数据源类型
	var datasourceType string
	if ext == ".csv" {
		datasourceType = nl2sqlCommon.DataSourceTypeCSV
	} else {
		datasourceType = nl2sqlCommon.DataSourceTypeExcel
	}

	// 2. 处理文件上传（使用现有的文件上传逻辑）
	fileName, _, _, file, err := common.HandleFileUpload(ctx, req.File, "")
	if err != nil {
		g.Log().Errorf(ctx, "HandleFileUpload failed: %v", err)
		return nil, fmt.Errorf("文件处理失败: %w", err)
	}

	// 3. 根据存储类型保存文件
	storageType := file_store.GetStorageType()
	var filePath string

	switch storageType {
	case file_store.StorageTypeLocal:
		// 保存到本地: upload/nl2sql/{filename}
		localPath, err := file_store.SaveFileToLocalNL2SQL(fileName, req.File)
		if err != nil {
			g.Log().Errorf(ctx, "SaveFileToLocal failed: %v", err)
			return nil, fmt.Errorf("文件保存失败: %w", err)
		}
		filePath = localPath
		g.Log().Infof(ctx, "File saved locally: %s", localPath)

	case file_store.StorageTypeRustFS:
		// 保存到RustFS: file/nl2sql/{filename}
		rustfsConfig := file_store.GetRustfsConfig()
		localPath, rustfsKey, err := file_store.SaveFileToRustFSNL2SQL(rustfsConfig.Client, rustfsConfig.BucketName, fileName, file)
		if err != nil {
			g.Log().Errorf(ctx, "SaveFileToRustFS failed: %v", err)
			return nil, fmt.Errorf("文件上传到RustFS失败: %w", err)
		}
		filePath = localPath // 使用本地路径，后续可以从本地或RustFS读取
		g.Log().Infof(ctx, "File saved to RustFS: %s, local: %s", rustfsKey, localPath)

	default:
		return nil, fmt.Errorf("未知的存储类型: %s", storageType)
	}

	// 4-7. 使用事务处理：创建数据源 -> 解析文件 -> 导入数据 -> 更新配置
	db := dao.GetDB()
	redisClient := cache.GetRedisClient()

	var datasourceID string
	var tableName string
	var rowCount int

	// 开始事务
	tx := db.Begin()
	if tx.Error != nil {
		gfile.Remove(filePath)
		return nil, fmt.Errorf("开始事务失败: %w", tx.Error)
	}

	// 使用defer确保事务处理
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			gfile.Remove(filePath)
			g.Log().Errorf(ctx, "Panic during upload: %v", r)
		}
	}()

	// 步骤4: 创建数据源记录
	serviceReq := &service.CreateDataSourceRequest{
		Name:      req.Name,
		Type:      datasourceType,
		Config:    map[string]interface{}{},
		CreatedBy: req.CreatedBy,
	}

	// 临时使用事务版本的service
	txService := service.NewNL2SQLService(tx, redisClient)
	ds, err := txService.CreateDataSource(ctx, serviceReq)
	if err != nil {
		tx.Rollback()
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to create datasource: %v", err)
		return nil, fmt.Errorf("创建数据源失败: %w", err)
	}
	datasourceID = ds.DatasourceID

	g.Log().Infof(ctx, "Datasource created: %s, now parsing and importing file", ds.DatasourceID)

	// 步骤5: 解析文件
	fileParser := parser.NewFileParser(tx)
	parsedTable, err := fileParser.ParseFile(ctx, filePath)
	if err != nil {
		tx.Rollback()
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to parse file: %v", err)
		return nil, fmt.Errorf("文件解析失败: %w", err)
	}

	tableName = parsedTable.TableName
	rowCount = len(parsedTable.Rows)

	// 步骤6: 导入表数据
	dbType := g.Cfg().MustGet(ctx, "database.default.type").String()
	tableImporter := parser.NewTableImporter(tx, dbType)

	if err := tableImporter.ImportTable(ctx, parsedTable); err != nil {
		tx.Rollback()
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to import table: %v", err)
		return nil, fmt.Errorf("数据导入失败: %w", err)
	}

	// 步骤8: 创建表的Schema元数据
	schemaBuilder := schema.NewSchemaBuilder(tx)
	displayName := req.DisplayName
	if displayName == "" {
		displayName = req.Name // 如果没有提供DisplayName，使用数据源名称
	}

	if err := schemaBuilder.BuildTableFromNL2SQLSchema(ctx, datasourceID, tableName, displayName, filePath); err != nil {
		tx.Rollback()
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to build table metadata: %v", err)
		return nil, fmt.Errorf("创建表元数据失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to commit transaction: %v", err)
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	g.Log().Infof(ctx, "NL2SQL file uploaded and imported successfully - DatasourceID: %s, Table: %s, Rows: %d",
		datasourceID, tableName, rowCount)

	return &v1.NL2SQLUploadFileRes{
		DatasourceID: datasourceID,
		FilePath:     filePath,
		Status:       nl2sqlCommon.DataSourceStatusActive,
		Message:      fmt.Sprintf("文件上传成功，数据已导入表 %s (%d 行)", tableName, rowCount),
	}, nil
}

// NL2SQLAddTable 添加表到现有数据源
func (c *ControllerV1) NL2SQLAddTable(ctx context.Context, req *v1.NL2SQLAddTableReq) (res *v1.NL2SQLAddTableRes, err error) {
	g.Log().Infof(ctx, "NL2SQLAddTable request - DatasourceID: %s, FileName: %s", req.DatasourceID, req.File.Filename)

	// 1. 验证文件类型
	ext := strings.ToLower(filepath.Ext(req.File.Filename))
	if ext != ".csv" && ext != ".xlsx" && ext != ".xls" {
		return nil, fmt.Errorf("不支持的文件类型，仅支持 CSV 和 Excel 文件")
	}

	// 2. 处理文件上传
	fileName, _, _, file, err := common.HandleFileUpload(ctx, req.File, "")
	if err != nil {
		g.Log().Errorf(ctx, "HandleFileUpload failed: %v", err)
		return nil, fmt.Errorf("文件处理失败: %w", err)
	}

	// 3. 根据存储类型保存文件
	storageType := file_store.GetStorageType()
	var filePath string

	switch storageType {
	case file_store.StorageTypeLocal:
		// 保存到本地: upload/nl2sql/{filename}
		localPath, err := file_store.SaveFileToLocalNL2SQL(fileName, req.File)
		if err != nil {
			g.Log().Errorf(ctx, "SaveFileToLocal failed: %v", err)
			return nil, fmt.Errorf("文件保存失败: %w", err)
		}
		filePath = localPath
		g.Log().Infof(ctx, "File saved locally: %s", localPath)

	case file_store.StorageTypeRustFS:
		// 保存到RustFS: file/nl2sql/{filename}
		rustfsConfig := file_store.GetRustfsConfig()
		localPath, rustfsKey, err := file_store.SaveFileToRustFSNL2SQL(rustfsConfig.Client, rustfsConfig.BucketName, fileName, file)
		if err != nil {
			g.Log().Errorf(ctx, "SaveFileToRustFS failed: %v", err)
			return nil, fmt.Errorf("文件上传到RustFS失败: %w", err)
		}
		filePath = localPath
		g.Log().Infof(ctx, "File saved to RustFS: %s, local: %s", rustfsKey, localPath)

	default:
		return nil, fmt.Errorf("未知的存储类型: %s", storageType)
	}

	// 4-6. 使用事务处理：解析文件 -> 导入数据 -> 添加表到数据源配置
	db := dao.GetDB()
	redisClient := cache.GetRedisClient()

	var tableName string
	var rowCount int

	// 开始事务
	tx := db.Begin()
	if tx.Error != nil {
		gfile.Remove(filePath)
		return nil, fmt.Errorf("开始事务失败: %w", tx.Error)
	}

	// 使用defer确保事务处理
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			gfile.Remove(filePath)
			g.Log().Errorf(ctx, "Panic during add table: %v", r)
		}
	}()

	// 步骤4: 解析文件
	fileParser := parser.NewFileParser(tx)
	parsedTable, err := fileParser.ParseFile(ctx, filePath)
	if err != nil {
		tx.Rollback()
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to parse file: %v", err)
		return nil, fmt.Errorf("文件解析失败: %w", err)
	}

	tableName = parsedTable.TableName
	rowCount = len(parsedTable.Rows)

	// 步骤5: 导入表数据
	dbType := g.Cfg().MustGet(ctx, "database.default.type").String()
	tableImporter := parser.NewTableImporter(tx, dbType)

	if err := tableImporter.ImportTable(ctx, parsedTable); err != nil {
		tx.Rollback()
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to import table: %v", err)
		return nil, fmt.Errorf("数据导入失败: %w", err)
	}

	// 步骤6: 添加表到数据源配置
	txService := service.NewNL2SQLService(tx, redisClient)
	if err := txService.AddTableToDataSource(ctx, req.DatasourceID, tableName); err != nil {
		tx.Rollback()
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to add table to datasource: %v", err)
		return nil, fmt.Errorf("添加表到数据源失败: %w", err)
	}

	// 步骤7: 创建表的Schema元数据（使用用户提供的DisplayName）
	schemaBuilder := schema.NewSchemaBuilder(tx)
	displayName := req.DisplayName
	if displayName == "" {
		// 如果没有提供DisplayName，从文件名提取一个合理的名称
		displayName = strings.TrimSuffix(req.File.Filename, filepath.Ext(req.File.Filename))
	}

	if err := schemaBuilder.BuildTableFromNL2SQLSchema(ctx, req.DatasourceID, tableName, displayName, filePath); err != nil {
		tx.Rollback()
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to build table metadata: %v", err)
		return nil, fmt.Errorf("创建表元数据失败: %w", err)
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		gfile.Remove(filePath)
		g.Log().Errorf(ctx, "Failed to commit transaction: %v", err)
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	g.Log().Infof(ctx, "NL2SQL table added successfully - DatasourceID: %s, Table: %s, Rows: %d",
		req.DatasourceID, tableName, rowCount)

	return &v1.NL2SQLAddTableRes{
		DatasourceID: req.DatasourceID,
		TableName:    tableName,
		RowCount:     rowCount,
		Status:       nl2sqlCommon.ExecutionStatusSuccess,
		Message:      fmt.Sprintf("表 %s 已成功添加到数据源 (%d 行)", tableName, rowCount),
	}, nil
}
