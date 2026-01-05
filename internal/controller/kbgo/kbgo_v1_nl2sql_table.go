package kbgo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/internal/dao"
	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	nl2sqlCache "github.com/Malowking/kbgo/nl2sql/cache"
	"github.com/Malowking/kbgo/nl2sql/service"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
	"gorm.io/gorm"
)

// ============ 表Schema相关操作 ============

// NL2SQLParseSchema 解析数据源Schema
func (c *ControllerV1) NL2SQLParseSchema(ctx context.Context, req *v1.NL2SQLParseSchemaReq) (res *v1.NL2SQLParseSchemaRes, err error) {
	g.Log().Infof(ctx, "NL2SQLParseSchema request - DatasourceID: %s, LLMModelID: %s, EmbeddingModelID: %s",
		req.DatasourceID, req.LLMModelID, req.EmbeddingModelID)

	// 获取数据库连接和Redis客户端
	db := dao.GetDB()
	redisClient := cache.GetRedisClient()
	nl2sqlService := service.NewNL2SQLService(db, redisClient)

	// 如果未提供 EmbeddingModelID，则从数据源获取
	embeddingModelID := req.EmbeddingModelID
	if embeddingModelID == "" {
		var datasource dbgorm.NL2SQLDataSource
		if err := db.First(&datasource, "id = ?", req.DatasourceID).Error; err != nil {
			g.Log().Errorf(ctx, "Failed to get datasource: %v", err)
			return nil, fmt.Errorf("数据源不存在")
		}
		embeddingModelID = datasource.EmbeddingModelID
		g.Log().Infof(ctx, "使用数据源绑定的 Embedding 模型: %s", embeddingModelID)
	}

	// 触发Schema解析任务，传递模型参数
	taskID, err := nl2sqlService.ParseDataSourceSchemaWithModels(ctx, req.DatasourceID, req.LLMModelID, embeddingModelID)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to parse schema: %v", err)
		return nil, err
	}

	g.Log().Infof(ctx, "Schema parsing task created: %s", taskID)
	return &v1.NL2SQLParseSchemaRes{
		TaskID: taskID,
	}, nil
}

// NL2SQLGetTask 获取任务状态
func (c *ControllerV1) NL2SQLGetTask(ctx context.Context, req *v1.NL2SQLGetTaskReq) (res *v1.NL2SQLGetTaskRes, err error) {
	g.Log().Infof(ctx, "NL2SQLGetTask request - TaskID: %s", req.TaskID)

	// 获取Redis客户端
	redisClient := cache.GetRedisClient()
	taskCache := nl2sqlCache.NewTaskCache(redisClient)

	// 从Redis查询任务状态
	task, err := taskCache.GetTask(ctx, req.TaskID)
	if err != nil {
		g.Log().Errorf(ctx, "Task not found: %v", err)
		return nil, fmt.Errorf("任务不存在: %w", err)
	}

	// 解析结果
	var result map[string]interface{}
	if task.Result != "" {
		if err := json.Unmarshal([]byte(task.Result), &result); err != nil {
			g.Log().Warningf(ctx, "Failed to unmarshal result: %v", err)
		}
	}

	return &v1.NL2SQLGetTaskRes{
		TaskID:      task.TaskID,
		Status:      task.Status,
		Progress:    task.Progress,
		CurrentStep: task.CurrentStep,
		ErrorMsg:    task.ErrorMsg,
		Result:      result,
	}, nil
}

// NL2SQLGetSchema 获取Schema信息
func (c *ControllerV1) NL2SQLGetSchema(ctx context.Context, req *v1.NL2SQLGetSchemaReq) (res *v1.NL2SQLGetSchemaRes, err error) {
	g.Log().Infof(ctx, "NL2SQLGetSchema request - DatasourceID: %s", req.DatasourceID)

	// 获取数据库连接
	db := dao.GetDB()

	// 1. 获取数据源
	var ds dbgorm.NL2SQLDataSource
	if err := db.First(&ds, "id = ?", req.DatasourceID).Error; err != nil {
		g.Log().Errorf(ctx, "Datasource not found: %v", err)
		return nil, fmt.Errorf("数据源不存在: %w", err)
	}

	// 2. 获取指标
	var metrics []dbgorm.NL2SQLMetric
	db.Where("datasource_id = ?", req.DatasourceID).Find(&metrics)

	// 初始化为空数组而不是 nil
	metricItems := make([]*v1.NL2SQLMetric, 0)
	for _, m := range metrics {
		metricItems = append(metricItems, &v1.NL2SQLMetric{
			MetricName:  m.Name,
			Description: m.Description,
			Formula:     m.Formula,
		})
	}

	// 3. 获取表和列
	var tables []dbgorm.NL2SQLTable
	db.Where("datasource_id = ?", req.DatasourceID).Find(&tables)

	// 初始化为空数组而不是 nil
	tableItems := make([]*v1.NL2SQLTableDetail, 0)
	for _, t := range tables {
		// 获取该表的列
		var columns []dbgorm.NL2SQLColumn
		db.Where("table_id = ?", t.ID).Find(&columns)

		// 初始化为空数组而不是 nil
		columnItems := make([]*v1.NL2SQLColumnDetail, 0)
		// 解析主键列表（从 t.PrimaryKey 字符串中）
		var primaryKeys []string
		if t.PrimaryKey != "" {
			// 假设主键字段用逗号分隔，例如 "id" 或 "user_id,order_id"
			primaryKeys = strings.Split(t.PrimaryKey, ",")
			// 去除空格
			for i := range primaryKeys {
				primaryKeys[i] = strings.TrimSpace(primaryKeys[i])
			}
		}

		for _, col := range columns {
			columnItems = append(columnItems, &v1.NL2SQLColumnDetail{
				ID:          col.ID,
				ColumnName:  col.ColumnName,
				DataType:    col.DataType,
				Description: col.Description,
				Nullable:    col.Nullable,
			})
		}

		tableItems = append(tableItems, &v1.NL2SQLTableDetail{
			ID:          t.ID,
			TableName:   t.Name,
			DisplayName: t.DisplayName,
			Description: t.Description,
			RowCount:    t.RowCountEstimate,
			Parsed:      t.Parsed,
			Columns:     columnItems,
			PrimaryKeys: primaryKeys,
		})
	}

	// 4. 获取表关系
	var relations []dbgorm.NL2SQLRelation
	db.Where("datasource_id = ?", req.DatasourceID).Find(&relations)

	// 初始化为空数组而不是 nil
	relationItems := make([]*v1.NL2SQLRelation, 0)
	for _, r := range relations {
		// 需要通过 FromTableID 和 ToTableID 查询表名
		var fromTable, toTable dbgorm.NL2SQLTable
		db.First(&fromTable, "id = ?", r.FromTableID)
		db.First(&toTable, "id = ?", r.ToTableID)

		relationItems = append(relationItems, &v1.NL2SQLRelation{
			ID:           r.ID,
			SourceTable:  fromTable.Name,
			TargetTable:  toTable.Name,
			RelationType: r.RelationType,
		})
	}

	g.Log().Infof(ctx, "Schema retrieved successfully - Tables: %d, Metrics: %d, Relations: %d",
		len(tableItems), len(metricItems), len(relationItems))

	return &v1.NL2SQLGetSchemaRes{
		SchemaID:  req.DatasourceID, // 使用 DatasourceID 作为 SchemaID
		Tables:    tableItems,
		Relations: relationItems,
		Metrics:   metricItems,
		Domains:   []*v1.NL2SQLDomain{}, // BusinessDomain 已删除，返回空数组
	}, nil
}

// ============ 表级别操作 ============

// deleteTableInternal 删除单个表的内部方法（在事务中执行，不提交事务）
func deleteTableInternal(ctx context.Context, tx *gorm.DB, datasourceID, tableID string) ([]string, error) {
	var filePaths []string

	// 1. 获取表信息（包括表名和文件路径）
	var table dbgorm.NL2SQLTable
	if err := tx.Where("id = ? AND datasource_id = ?", tableID, datasourceID).First(&table).Error; err != nil {
		g.Log().Errorf(ctx, "Table not found: %v", err)
		return nil, fmt.Errorf("表不存在: %w", err)
	}

	tableName := table.Name
	filePath := table.FilePath

	g.Log().Infof(ctx, "Deleting table: %s (ID: %s, FilePath: %s)", tableName, tableID, filePath)

	// 2. 删除 nl2sql_columns（表的列信息）
	if err := tx.Where("table_id = ?", tableID).Delete(&dbgorm.NL2SQLColumn{}).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to delete columns: %v", err)
		return nil, fmt.Errorf("删除列信息失败: %w", err)
	}
	g.Log().Infof(ctx, "Deleted columns for table %s", tableName)

	// 3. 删除 nl2sql_relations（涉及该表的关系）
	if err := tx.Where("from_table_id = ? OR to_table_id = ?", tableID, tableID).Delete(&dbgorm.NL2SQLRelation{}).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to delete relations: %v", err)
		return nil, fmt.Errorf("删除表关系失败: %w", err)
	}
	g.Log().Infof(ctx, "Deleted relations for table %s", tableName)

	// 4. 删除 nl2sql_vector_docs（表的向量文档）
	if err := tx.Where("datasource_id = ? AND entity_type = 'table' AND entity_id = ?", datasourceID, tableID).Delete(&dbgorm.NL2SQLVectorDoc{}).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to delete vector docs for table: %v", err)
		return nil, fmt.Errorf("删除表向量文档失败: %w", err)
	}

	// 删除该表的列对应的向量文档
	var columns []dbgorm.NL2SQLColumn
	if err := tx.Where("table_id = ?", tableID).Find(&columns).Error; err == nil {
		for _, col := range columns {
			tx.Where("datasource_id = ? AND entity_type = 'column' AND entity_id = ?", datasourceID, col.ID).Delete(&dbgorm.NL2SQLVectorDoc{})
		}
	}
	g.Log().Infof(ctx, "Deleted vector docs for table %s", tableName)

	// 5. 删除 nl2sql_metrics（引用该表的指标）
	if err := tx.Where("datasource_id = ?", datasourceID).Delete(&dbgorm.NL2SQLMetric{}).Error; err != nil {
		g.Log().Warningf(ctx, "Failed to delete metrics (non-fatal): %v", err)
	}
	g.Log().Infof(ctx, "Deleted metrics related to table %s", tableName)

	// 6. 删除 nl2sql schema 中的实际数据表
	dropSQL := fmt.Sprintf("DROP TABLE IF EXISTS nl2sql.%s CASCADE", tableName)
	if err := tx.Exec(dropSQL).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to drop table %s: %v", tableName, err)
		return nil, fmt.Errorf("删除数据表失败: %w", err)
	}
	g.Log().Infof(ctx, "Dropped table: nl2sql.%s", tableName)

	// 7. 删除 nl2sql_tables 表记录
	if err := tx.Delete(&table).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to delete table metadata: %v", err)
		return nil, fmt.Errorf("删除表元数据失败: %w", err)
	}
	g.Log().Infof(ctx, "Deleted table metadata for %s", tableName)

	// 收集文件路径
	if filePath != "" {
		filePaths = append(filePaths, filePath)
	}

	return filePaths, nil
}

// NL2SQLDeleteTable 删除数据源中的表（级联删除所有关联数据）
func (c *ControllerV1) NL2SQLDeleteTable(ctx context.Context, req *v1.NL2SQLDeleteTableReq) (res *v1.NL2SQLDeleteTableRes, err error) {
	g.Log().Infof(ctx, "NL2SQLDeleteTable request - DatasourceID: %s, TableID: %s", req.DatasourceID, req.TableID)

	db := dao.GetDB()

	// 开始事务
	tx := db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("开始事务失败: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			g.Log().Errorf(ctx, "Panic during delete table: %v", r)
		}
	}()

	// 调用内部删除方法
	filePaths, err := deleteTableInternal(ctx, tx, req.DatasourceID, req.TableID)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// 提交事务
	if err := tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "Failed to commit transaction: %v", err)
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	// 删除文件（在事务外，不影响数据库操作）
	for _, filePath := range filePaths {
		if err := gfile.Remove(filePath); err != nil {
			g.Log().Warningf(ctx, "Failed to remove file %s: %v (non-fatal)", filePath, err)
		} else {
			g.Log().Infof(ctx, "Removed file: %s", filePath)
		}
	}

	g.Log().Infof(ctx, "Table deleted successfully: %s", req.TableID)
	return &v1.NL2SQLDeleteTableRes{
		Success: true,
		Message: "表已成功删除",
	}, nil
}
