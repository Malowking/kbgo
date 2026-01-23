package kbgo

import (
	"context"
	"encoding/json"
	"fmt"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	dbgorm "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/datatypes"
)

// NL2SQLCreateMetric 创建预定义指标
func (c *ControllerV1) NL2SQLCreateMetric(ctx context.Context, req *v1.NL2SQLCreateMetricReq) (res *v1.NL2SQLCreateMetricRes, err error) {
	db := dao.GetDB()

	// 1. 验证数据源是否存在
	var ds dbgorm.NL2SQLDataSource
	if err := db.First(&ds, "id = ?", req.DatasourceID).Error; err != nil {
		return nil, fmt.Errorf("数据源不存在")
	}

	// 2. 检查 MetricCode 是否已存在
	var existingMetric dbgorm.NL2SQLMetric
	if err := db.Where("datasource_id = ? AND metric_id = ?", req.DatasourceID, req.MetricCode).First(&existingMetric).Error; err == nil {
		return nil, fmt.Errorf("指标代码已存在: %s", req.MetricCode)
	}

	// 3. 序列化默认过滤条件
	var defaultFiltersJSON []byte
	if req.DefaultFilters != nil {
		defaultFiltersJSON, _ = json.Marshal(req.DefaultFilters)
	} else {
		defaultFiltersJSON = []byte("{}")
	}

	// 4. 创建 Metric 记录
	metric := &dbgorm.NL2SQLMetric{
		DatasourceID:   req.DatasourceID,
		MetricCode:     req.MetricCode,
		Name:           req.Name,
		Description:    req.Description,
		Formula:        req.Formula,
		DefaultFilters: datatypes.JSON(defaultFiltersJSON),
		TimeColumn:     req.TimeColumn,
	}

	if err := db.Create(metric).Error; err != nil {
		return nil, fmt.Errorf("创建指标失败: %w", err)
	}

	g.Log().Infof(ctx, "Metric created: %s (%s)", metric.Name, metric.MetricCode)

	return &v1.NL2SQLCreateMetricRes{
		MetricID: metric.ID,
		Message:  "指标创建成功",
	}, nil
}

// NL2SQLListMetrics 查询指标列表
func (c *ControllerV1) NL2SQLListMetrics(ctx context.Context, req *v1.NL2SQLListMetricsReq) (res *v1.NL2SQLListMetricsRes, err error) {
	db := dao.GetDB()

	var metrics []dbgorm.NL2SQLMetric
	if err := db.Where("datasource_id = ?", req.DatasourceID).Find(&metrics).Error; err != nil {
		return nil, fmt.Errorf("查询指标失败: %w", err)
	}

	// 转换为返回格式
	metricInfos := make([]v1.NL2SQLMetricInfo, 0, len(metrics))
	for _, m := range metrics {
		var filters map[string]interface{}
		if len(m.DefaultFilters) > 0 {
			json.Unmarshal(m.DefaultFilters, &filters)
		}

		metricInfos = append(metricInfos, v1.NL2SQLMetricInfo{
			ID:             m.ID,
			MetricCode:     m.MetricCode,
			Name:           m.Name,
			Description:    m.Description,
			Formula:        m.Formula,
			DefaultFilters: filters,
			TimeColumn:     m.TimeColumn,
			CreateTime:     m.CreateTime.Format("2006-01-02 15:04:05"),
		})
	}

	return &v1.NL2SQLListMetricsRes{
		Metrics: metricInfos,
	}, nil
}

// NL2SQLDeleteMetric 删除指标
func (c *ControllerV1) NL2SQLDeleteMetric(ctx context.Context, req *v1.NL2SQLDeleteMetricReq) (res *v1.NL2SQLDeleteMetricRes, err error) {
	db := dao.GetDB()

	// 查找指标
	var metric dbgorm.NL2SQLMetric
	if err := db.First(&metric, "id = ?", req.MetricID).Error; err != nil {
		return &v1.NL2SQLDeleteMetricRes{
			Success: false,
			Message: "指标不存在",
		}, nil
	}

	// 删除指标
	if err := db.Delete(&metric).Error; err != nil {
		return nil, fmt.Errorf("删除指标失败: %w", err)
	}

	g.Log().Infof(ctx, "Metric deleted: %s (%s)", metric.Name, metric.MetricCode)

	return &v1.NL2SQLDeleteMetricRes{
		Success: true,
		Message: "指标删除成功",
	}, nil
}
