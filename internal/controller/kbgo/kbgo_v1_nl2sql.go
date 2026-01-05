package kbgo

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/cache"
	"github.com/Malowking/kbgo/internal/dao"
	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/Malowking/kbgo/nl2sql/service"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gfile"
)

// ============ 数据源级别操作 ============

// NL2SQLCreateDataSource 创建数据源
func (c *ControllerV1) NL2SQLCreateDataSource(ctx context.Context, req *v1.NL2SQLCreateDataSourceReq) (res *v1.NL2SQLCreateDataSourceRes, err error) {
	g.Log().Infof(ctx, "NL2SQLCreateDataSource request - Name: %s, Type: %s, DBType: %s, EmbeddingModelID: %s",
		req.Name, req.Type, req.DBType, req.EmbeddingModelID)

	// 获取数据库连接和Redis客户端
	db := dao.GetDB()
	redisClient := cache.GetRedisClient()
	nl2sqlService := service.NewNL2SQLService(db, redisClient)

	// 调用服务层创建数据源（包含向量表创建）
	serviceReq := &service.CreateDataSourceRequest{
		Name:             req.Name,
		Type:             req.Type,
		DBType:           req.DBType,
		Config:           req.Config,
		CreatedBy:        req.CreatedBy,
		EmbeddingModelID: req.EmbeddingModelID,
	}

	dsResp, err := nl2sqlService.CreateDataSource(ctx, serviceReq)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to create datasource: %v", err)
		return nil, err
	}

	g.Log().Infof(ctx, "Datasource created successfully: %s, Collection: %s", dsResp.DatasourceID, dsResp.CollectionName)
	return &v1.NL2SQLCreateDataSourceRes{
		DatasourceID:     dsResp.DatasourceID,
		Status:           dsResp.Status,
		CollectionName:   dsResp.CollectionName,
		VectorStoreReady: dsResp.VectorStoreReady,
	}, nil
}

// NL2SQLListDataSources 列出数据源
func (c *ControllerV1) NL2SQLListDataSources(ctx context.Context, req *v1.NL2SQLListDataSourcesReq) (res *v1.NL2SQLListDataSourcesRes, err error) {
	g.Log().Infof(ctx, "NL2SQLListDataSources request - Status: %s, Page: %d, Size: %d",
		req.Status, req.Page, req.Size)

	// 获取数据库连接
	db := dao.GetDB()

	// 构建查询
	query := db.Model(&dbgorm.NL2SQLDataSource{})

	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

	// 计算总数
	var total int64
	if err := query.Count(&total).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to count datasources: %v", err)
		return nil, err
	}

	// 分页查询
	var datasources []dbgorm.NL2SQLDataSource
	offset := (req.Page - 1) * req.Size
	if err := query.Offset(offset).Limit(req.Size).Order("create_time DESC").Find(&datasources).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to list datasources: %v", err)
		return nil, err
	}

	var items []*v1.NL2SQLDataSourceItem
	for _, ds := range datasources {
		items = append(items, &v1.NL2SQLDataSourceItem{
			ID:               ds.ID,
			Name:             ds.Name,
			Type:             ds.Type,
			DBType:           ds.DBType,
			Status:           ds.Status,
			EmbeddingModelID: ds.EmbeddingModelID,
			CreatedAt:        ds.CreateTime.Format("2006-01-02 15:04:05"),
			UpdatedAt:        ds.UpdateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &v1.NL2SQLListDataSourcesRes{
		List:  items,
		Total: total,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

// NL2SQLDeleteDataSource 删除数据源
func (c *ControllerV1) NL2SQLDeleteDataSource(ctx context.Context, req *v1.NL2SQLDeleteDataSourceReq) (res *v1.NL2SQLDeleteDataSourceRes, err error) {
	g.Log().Infof(ctx, "NL2SQLDeleteDataSource request - DatasourceID: %s", req.DatasourceID)

	// 获取数据库连接
	db := dao.GetDB()

	// 1. 先获取数据源信息，用于后续清理
	var ds dbgorm.NL2SQLDataSource
	if err := db.First(&ds, "id = ?", req.DatasourceID).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to find datasource: %v", err)
		return nil, fmt.Errorf("数据源不存在: %w", err)
	}

	// 2. 获取该数据源下的所有表
	var tables []dbgorm.NL2SQLTable
	if err := db.Where("datasource_id = ?", req.DatasourceID).Find(&tables).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to get tables: %v", err)
		return nil, fmt.Errorf("获取表列表失败: %w", err)
	}

	// 3. 开始事务删除
	tx := db.Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("开始事务失败: %w", tx.Error)
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			g.Log().Errorf(ctx, "Panic during delete: %v", r)
		}
	}()

	// 4. 使用复用的方法删除所有表（在同一个事务中）
	var allFilePaths []string
	for _, table := range tables {
		g.Log().Infof(ctx, "Deleting table %s from datasource %s", table.Name, req.DatasourceID)
		filePaths, err := deleteTableInternal(ctx, tx, req.DatasourceID, table.ID)
		if err != nil {
			tx.Rollback()
			g.Log().Errorf(ctx, "Failed to delete table %s: %v", table.Name, err)
			return nil, fmt.Errorf("删除表 %s 失败: %w", table.Name, err)
		}
		allFilePaths = append(allFilePaths, filePaths...)
	}

	// 5. 删除数据源记录
	if err := tx.Delete(&dbgorm.NL2SQLDataSource{}, "id = ?", req.DatasourceID).Error; err != nil {
		tx.Rollback()
		g.Log().Errorf(ctx, "Failed to delete datasource: %v", err)
		return nil, fmt.Errorf("删除数据源失败: %w", err)
	}

	// 6. 提交事务
	if err := tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "Failed to commit transaction: %v", err)
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	// 7. 删除文件（在事务外）
	for _, filePath := range allFilePaths {
		if err := gfile.Remove(filePath); err != nil {
			g.Log().Warningf(ctx, "Failed to remove file %s: %v (non-fatal)", filePath, err)
		} else {
			g.Log().Infof(ctx, "Removed file: %s", filePath)
		}
	}

	// 8. 删除向量库中的collection（在事务外）
	collectionName := fmt.Sprintf("nl2sql_%s", req.DatasourceID)
	redisClient := cache.GetRedisClient()
	nl2sqlService := service.NewNL2SQLService(db, redisClient)
	if err := nl2sqlService.DeleteVectorCollection(ctx, collectionName); err != nil {
		g.Log().Warningf(ctx, "Failed to delete vector collection %s: %v", collectionName, err)
	} else {
		g.Log().Infof(ctx, "Deleted vector collection: %s", collectionName)
	}

	g.Log().Infof(ctx, "Datasource deleted successfully: %s", req.DatasourceID)
	return &v1.NL2SQLDeleteDataSourceRes{
		Success: true,
	}, nil
}

// ============ 查询级别操作 ============

// NL2SQLQuery 执行NL2SQL查询
func (c *ControllerV1) NL2SQLQuery(ctx context.Context, req *v1.NL2SQLQueryReq) (res *v1.NL2SQLQueryRes, err error) {
	g.Log().Infof(ctx, "NL2SQLQuery request - DatasourceID: %s, Question: %s, SessionID: %s",
		req.DatasourceID, req.Question, req.SessionID)

	// 获取数据库连接和Redis客户端
	db := dao.GetDB()
	redisClient := cache.GetRedisClient()
	nl2sqlService := service.NewNL2SQLService(db, redisClient)

	// TODO: 集成LLM和向量搜索适配器
	// 目前先调用基础的Query方法，后续需要：
	// 1. 获取数据源关联的AgentPreset
	// 2. 根据AgentPreset配置创建LLMAdapter和VectorSearchAdapter
	// 3. 调用QueryWithAdapters方法

	// 调用服务层执行查询
	serviceReq := &service.QueryRequest{
		DatasourceID: req.DatasourceID,
		Question:     req.Question,
		SessionID:    req.SessionID,
	}

	queryResp, err := nl2sqlService.Query(ctx, serviceReq)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to execute NL2SQL query: %v", err)
		// 即使查询失败，也返回一个包含错误信息的响应
		return &v1.NL2SQLQueryRes{
			Error: err.Error(),
		}, nil
	}

	// 转换结果
	var result *v1.NL2SQLQueryResult
	if queryResp.Result != nil {
		result = &v1.NL2SQLQueryResult{
			Columns:  queryResp.Result.Columns,
			Data:     queryResp.Result.Data,
			RowCount: queryResp.Result.RowCount,
		}
	}

	g.Log().Infof(ctx, "NL2SQL query executed successfully - QueryLogID: %s", queryResp.QueryLogID)
	return &v1.NL2SQLQueryRes{
		QueryLogID:  queryResp.QueryLogID,
		SQL:         queryResp.SQL,
		Result:      result,
		Explanation: queryResp.Explanation,
		Error:       queryResp.Error,
	}, nil
}

// NL2SQLFeedback 提交查询反馈
func (c *ControllerV1) NL2SQLFeedback(ctx context.Context, req *v1.NL2SQLFeedbackReq) (res *v1.NL2SQLFeedbackRes, err error) {
	g.Log().Infof(ctx, "NL2SQLFeedback request - QueryLogID: %s, Feedback: %s",
		req.QueryLogID, req.Feedback)

	// 获取数据库连接
	db := dao.GetDB()

	// 更新查询日志的反馈信息
	updates := map[string]interface{}{
		"user_feedback": req.Feedback,
	}
	if req.Comment != "" {
		updates["feedback_comment"] = req.Comment
	}

	if err := db.Model(&dbgorm.NL2SQLQueryLog{}).
		Where("id = ?", req.QueryLogID).
		Updates(updates).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to update feedback: %v", err)
		return nil, err
	}

	g.Log().Infof(ctx, "Feedback saved successfully for QueryLogID: %s", req.QueryLogID)
	return &v1.NL2SQLFeedbackRes{
		Success: true,
	}, nil

}
