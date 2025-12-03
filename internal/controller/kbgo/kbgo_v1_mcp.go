package kbgo

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/mcp/client"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

// MCPRegistryCreate 创建MCP服务注册
func (c *ControllerV1) MCPRegistryCreate(ctx context.Context, req *v1.MCPRegistryCreateReq) (res *v1.MCPRegistryCreateRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPRegistryCreate request received - Name: %s, Description: %s, Endpoint: %s, Timeout: %v",
		req.Name, req.Description, req.Endpoint, req.Timeout)

	// 检查名称是否已存在
	exists, err := dao.MCPRegistry.Exists(ctx, req.Name)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to check MCP registry existence")
	}
	if exists {
		return nil, gerror.Newf("MCP service name '%s' already exists", req.Name)
	}

	// 生成ID
	id := strings.ReplaceAll(uuid.New().String(), "-", "")

	// 默认超时时间
	timeout := 30
	if req.Timeout != nil {
		timeout = *req.Timeout
	}

	// 创建注册记录
	registry := &gormModel.MCPRegistry{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Endpoint:    req.Endpoint,
		ApiKey:      req.ApiKey,
		Headers:     req.Headers,
		Timeout:     timeout,
		Status:      1,    // 默认启用
		Tools:       "[]", // 默认空工具列表
	}

	if err := dao.MCPRegistry.Create(ctx, registry); err != nil {
		return nil, gerror.Wrap(err, "failed to create MCP registry")
	}

	return &v1.MCPRegistryCreateRes{Id: id}, nil
}

// MCPRegistryUpdate 更新MCP服务注册
func (c *ControllerV1) MCPRegistryUpdate(ctx context.Context, req *v1.MCPRegistryUpdateReq) (res *v1.MCPRegistryUpdateRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPRegistryUpdate request received - Id: %s, Name: %v, Description: %v, Endpoint: %v, Status: %v",
		req.Id, req.Name, req.Description, req.Endpoint, req.Status)

	// 查询现有记录
	registry, err := dao.MCPRegistry.GetByID(ctx, req.Id)
	if err != nil {
		return nil, gerror.Wrap(err, "MCP service not found")
	}

	// 如果更新名称，检查是否重名
	if req.Name != nil && *req.Name != registry.Name {
		exists, err := dao.MCPRegistry.Exists(ctx, *req.Name, req.Id)
		if err != nil {
			return nil, gerror.Wrap(err, "failed to check MCP registry existence")
		}
		if exists {
			return nil, gerror.Newf("MCP service name '%s' already exists", *req.Name)
		}
		registry.Name = *req.Name
	}

	// 更新字段
	if req.Description != nil {
		registry.Description = *req.Description
	}
	if req.Endpoint != nil {
		registry.Endpoint = *req.Endpoint
	}
	if req.ApiKey != nil {
		registry.ApiKey = *req.ApiKey
	}
	if req.Headers != nil {
		registry.Headers = *req.Headers
	}
	if req.Timeout != nil {
		registry.Timeout = *req.Timeout
	}
	if req.Status != nil {
		registry.Status = *req.Status
	}

	if err := dao.MCPRegistry.Update(ctx, registry); err != nil {
		return nil, gerror.Wrap(err, "failed to update MCP registry")
	}

	return &v1.MCPRegistryUpdateRes{}, nil
}

// MCPRegistryDelete 删除MCP服务注册
func (c *ControllerV1) MCPRegistryDelete(ctx context.Context, req *v1.MCPRegistryDeleteReq) (res *v1.MCPRegistryDeleteRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPRegistryDelete request received - Id: %s", req.Id)

	// 检查是否存在
	_, err = dao.MCPRegistry.GetByID(ctx, req.Id)
	if err != nil {
		return nil, gerror.Wrap(err, "MCP service not found")
	}

	// 删除注册记录
	if err := dao.MCPRegistry.Delete(ctx, req.Id); err != nil {
		return nil, gerror.Wrap(err, "failed to delete MCP registry")
	}

	return &v1.MCPRegistryDeleteRes{}, nil
}

// MCPRegistryGetOne 获取单个MCP服务
func (c *ControllerV1) MCPRegistryGetOne(ctx context.Context, req *v1.MCPRegistryGetOneReq) (res *v1.MCPRegistryGetOneRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPRegistryGetOne request received - Id: %s", req.Id)

	registry, err := dao.MCPRegistry.GetByID(ctx, req.Id)
	if err != nil {
		return nil, gerror.Wrap(err, "MCP service not found")
	}

	// 脱敏API Key
	maskedApiKey := ""
	if registry.ApiKey != "" {
		if len(registry.ApiKey) > 8 {
			maskedApiKey = registry.ApiKey[:4] + "****" + registry.ApiKey[len(registry.ApiKey)-4:]
		} else {
			maskedApiKey = "****"
		}
	}

	return &v1.MCPRegistryGetOneRes{
		Id:          registry.ID,
		Name:        registry.Name,
		Description: registry.Description,
		Endpoint:    registry.Endpoint,
		ApiKey:      maskedApiKey,
		Headers:     registry.Headers,
		Timeout:     registry.Timeout,
		Status:      registry.Status,
		CreateTime:  registry.CreateTime.Format(time.RFC3339),
		UpdateTime:  registry.UpdateTime.Format(time.RFC3339),
	}, nil
}

// MCPRegistryGetList 获取MCP服务列表
func (c *ControllerV1) MCPRegistryGetList(ctx context.Context, req *v1.MCPRegistryGetListReq) (res *v1.MCPRegistryGetListRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPRegistryGetList request received - Status: %v, Page: %d, PageSize: %d",
		req.Status, req.Page, req.PageSize)

	registries, total, err := dao.MCPRegistry.List(ctx, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to get MCP registry list")
	}

	items := make([]*v1.MCPRegistryItem, 0, len(registries))
	for _, r := range registries {
		items = append(items, &v1.MCPRegistryItem{
			Id:          r.ID,
			Name:        r.Name,
			Description: r.Description,
			Endpoint:    r.Endpoint,
			Timeout:     r.Timeout,
			Status:      r.Status,
			CreateTime:  r.CreateTime.Format(time.RFC3339),
			UpdateTime:  r.UpdateTime.Format(time.RFC3339),
		})
	}

	return &v1.MCPRegistryGetListRes{
		List:  items,
		Total: total,
		Page:  req.Page,
	}, nil
}

// MCPRegistryUpdateStatus 更新MCP服务状态
func (c *ControllerV1) MCPRegistryUpdateStatus(ctx context.Context, req *v1.MCPRegistryUpdateStatusReq) (res *v1.MCPRegistryUpdateStatusRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPRegistryUpdateStatus request received - Id: %s, Status: %d", req.Id, req.Status)

	if err := dao.MCPRegistry.UpdateStatus(ctx, req.Id, req.Status); err != nil {
		return nil, gerror.Wrap(err, "failed to update MCP registry status")
	}
	return &v1.MCPRegistryUpdateStatusRes{}, nil
}

// MCPRegistryTest 测试MCP服务连通性
func (c *ControllerV1) MCPRegistryTest(ctx context.Context, req *v1.MCPRegistryTestReq) (res *v1.MCPRegistryTestRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPRegistryTest request received - Id: %s", req.Id)

	registry, err := dao.MCPRegistry.GetByID(ctx, req.Id)
	if err != nil {
		return &v1.MCPRegistryTestRes{
			Success: false,
			Message: "MCP service not found",
		}, nil
	}

	// 创建客户端并测试连接
	mcpClient := client.NewMCPClient(registry)

	// 初始化连接
	err = mcpClient.Initialize(ctx, map[string]interface{}{
		"name":    "kbgo",
		"version": "1.0.0",
	})
	if err != nil {
		return &v1.MCPRegistryTestRes{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &v1.MCPRegistryTestRes{
		Success: true,
		Message: "Connection successful",
	}, nil
}

// MCPListTools 列出MCP服务的所有工具
func (c *ControllerV1) MCPListTools(ctx context.Context, req *v1.MCPListToolsReq) (res *v1.MCPListToolsRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPListTools request received - Id: %s, Cached: %v, CacheTTL: %v",
		req.Id, req.Cached, req.CacheTTL)

	registry, err := dao.MCPRegistry.GetByID(ctx, req.Id)
	if err != nil {
		return nil, gerror.Wrap(err, "MCP service not found")
	}

	// 检查是否使用缓存
	useCache := true
	if req.Cached != nil {
		useCache = *req.Cached
	}

	// 如果使用缓存且数据库中有工具列表，则直接返回
	if useCache && registry.Tools != "" && registry.Tools != "[]" {
		var tools []v1.MCPToolInfo
		if err := json.Unmarshal([]byte(registry.Tools), &tools); err == nil {
			// 检查缓存是否过期（简单实现，可根据需要增强）
			// cacheTTL := 300 // 默认5分钟
			// if req.CacheTTL != nil {
			// 	cacheTTL = *req.CacheTTL
			// }

			// 这里可以添加更复杂的缓存过期逻辑
			// 简单起见，我们直接使用缓存数据
			return &v1.MCPListToolsRes{Tools: tools}, nil
		}
	}

	// 创建客户端
	mcpClient := client.NewMCPClient(registry)

	// 初始化连接
	err = mcpClient.Initialize(ctx, map[string]interface{}{
		"name":    "kbgo",
		"version": "1.0.0",
	})
	if err != nil {
		return nil, gerror.Wrap(err, "failed to initialize MCP connection")
	}

	// 获取工具列表
	tools, err := mcpClient.ListTools(ctx)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to list MCP tools")
	}

	// 转换为响应格式
	toolInfos := make([]v1.MCPToolInfo, 0, len(tools))
	for _, tool := range tools {
		toolInfos = append(toolInfos, v1.MCPToolInfo{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	// 将工具列表保存到数据库中
	toolsJSON, err := json.Marshal(toolInfos)
	if err == nil {
		registry.Tools = string(toolsJSON)
		if updateErr := dao.MCPRegistry.Update(ctx, registry); updateErr != nil {
			g.Log().Errorf(ctx, "Failed to update MCP registry tools: %v", updateErr)
		}
	}

	return &v1.MCPListToolsRes{Tools: toolInfos}, nil
}

// MCPCallTool 调用MCP工具
func (c *ControllerV1) MCPCallTool(ctx context.Context, req *v1.MCPCallToolReq) (res *v1.MCPCallToolRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPCallTool request received - RegistryID: %s, ToolName: %s, ConversationID: %s",
		req.RegistryID, req.ToolName, req.ConversationID)

	startTime := time.Now()

	// 查询MCP服务（支持ID或名称）
	var registry *gormModel.MCPRegistry
	if strings.HasPrefix(req.RegistryID, "mcp_") {
		registry, err = dao.MCPRegistry.GetByID(ctx, req.RegistryID)
	} else {
		registry, err = dao.MCPRegistry.GetByName(ctx, req.RegistryID)
	}
	if err != nil {
		return nil, gerror.Wrap(err, "MCP service not found")
	}

	// 检查服务是否启用
	if registry.Status != 1 {
		return nil, gerror.New("MCP service is disabled")
	}

	// 创建客户端
	mcpClient := client.NewMCPClient(registry)

	// 初始化连接
	err = mcpClient.Initialize(ctx, map[string]interface{}{
		"name":    "kbgo",
		"version": "1.0.0",
	})
	if err != nil {
		return nil, gerror.Wrap(err, "failed to initialize MCP connection")
	}

	// 调用工具
	result, err := mcpClient.CallTool(ctx, req.ToolName, req.Arguments)

	// 计算耗时
	duration := int(time.Since(startTime).Milliseconds())

	// 序列化请求和响应
	reqPayload, _ := json.Marshal(req.Arguments)
	respPayload, _ := json.Marshal(result)

	// 记录调用日志
	logStatus := int8(1) // 成功
	errorMsg := ""
	if err != nil {
		logStatus = 0 // 失败
		errorMsg = err.Error()
	}

	logID := strings.ReplaceAll(uuid.New().String(), "-", "")
	callLog := &gormModel.MCPCallLog{
		ID:              logID,
		ConversationID:  req.ConversationID,
		MCPRegistryID:   registry.ID,
		MCPServiceName:  registry.Name,
		ToolName:        req.ToolName,
		RequestPayload:  string(reqPayload),
		ResponsePayload: string(respPayload),
		Status:          logStatus,
		ErrorMessage:    errorMsg,
		Duration:        duration,
	}

	if logErr := dao.MCPCallLog.Create(ctx, callLog); logErr != nil {
		g.Log().Errorf(ctx, "Failed to create MCP call log: %v", logErr)
	}

	// 如果调用失败，返回错误
	if err != nil {
		return nil, gerror.Wrap(err, "failed to call MCP tool")
	}

	// 转换响应格式
	content := make([]v1.MCPContentItem, 0, len(result.Content))
	for _, c := range result.Content {
		content = append(content, v1.MCPContentItem{
			Type: c.Type,
			Text: c.Text,
			Data: c.Data,
		})
	}

	return &v1.MCPCallToolRes{
		Content: content,
		IsError: result.IsError,
		LogID:   logID,
	}, nil
}

// MCPCallLogGetList 获取MCP调用日志列表
func (c *ControllerV1) MCPCallLogGetList(ctx context.Context, req *v1.MCPCallLogGetListReq) (res *v1.MCPCallLogGetListRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPCallLogGetList request received - ConversationID: %v, RegistryID: %v, ServiceName: %v, ToolName: %v, Status: %v, StartTime: %v, EndTime: %v, Page: %d, PageSize: %d",
		req.ConversationID, req.RegistryID, req.ServiceName, req.ToolName, req.Status, req.StartTime, req.EndTime, req.Page, req.PageSize)

	// 构建过滤条件
	filter := &dao.MCPCallLogFilter{}

	if req.ConversationID != nil {
		filter.ConversationID = *req.ConversationID
	}
	if req.RegistryID != nil {
		filter.MCPRegistryID = *req.RegistryID
	}
	if req.ServiceName != nil {
		filter.MCPServiceName = *req.ServiceName
	}
	if req.ToolName != nil {
		filter.ToolName = *req.ToolName
	}
	if req.Status != nil {
		filter.Status = req.Status
	}

	// 解析时间
	if req.StartTime != nil {
		t, err := time.Parse(time.RFC3339, *req.StartTime)
		if err == nil {
			filter.StartTime = &t
		}
	}
	if req.EndTime != nil {
		t, err := time.Parse(time.RFC3339, *req.EndTime)
		if err == nil {
			filter.EndTime = &t
		}
	}

	// 查询日志
	logs, total, err := dao.MCPCallLog.List(ctx, filter, req.Page, req.PageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to get MCP call logs")
	}

	// 转换为响应格式
	items := make([]*v1.MCPCallLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, &v1.MCPCallLogItem{
			Id:              log.ID,
			ConversationID:  log.ConversationID,
			MCPRegistryID:   log.MCPRegistryID,
			MCPServiceName:  log.MCPServiceName,
			ToolName:        log.ToolName,
			RequestPayload:  log.RequestPayload,
			ResponsePayload: log.ResponsePayload,
			Status:          log.Status,
			ErrorMessage:    log.ErrorMessage,
			Duration:        log.Duration,
			CreateTime:      log.CreateTime.Format(time.RFC3339),
		})
	}

	return &v1.MCPCallLogGetListRes{
		List:  items,
		Total: total,
		Page:  req.Page,
	}, nil
}

// MCPCallLogGetByConversation 根据对话ID获取MCP调用日志
func (c *ControllerV1) MCPCallLogGetByConversation(ctx context.Context, req *v1.MCPCallLogGetByConversationReq) (res *v1.MCPCallLogGetByConversationRes, err error) {
	logs, total, err := dao.MCPCallLog.ListByConversationID(ctx, req.ConversationID, req.Page, req.PageSize)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to get MCP call logs")
	}

	items := make([]*v1.MCPCallLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, &v1.MCPCallLogItem{
			Id:              log.ID,
			ConversationID:  log.ConversationID,
			MCPRegistryID:   log.MCPRegistryID,
			MCPServiceName:  log.MCPServiceName,
			ToolName:        log.ToolName,
			RequestPayload:  log.RequestPayload,
			ResponsePayload: log.ResponsePayload,
			Status:          log.Status,
			ErrorMessage:    log.ErrorMessage,
			Duration:        log.Duration,
			CreateTime:      log.CreateTime.Format(time.RFC3339),
		})
	}

	return &v1.MCPCallLogGetByConversationRes{
		List:  items,
		Total: total,
		Page:  req.Page,
	}, nil
}

// MCPRegistryStats 获取MCP服务统计信息
func (c *ControllerV1) MCPRegistryStats(ctx context.Context, req *v1.MCPRegistryStatsReq) (res *v1.MCPRegistryStatsRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "MCPRegistryStats request received - Id: %s", req.Id)

	stats, err := dao.MCPCallLog.GetStatsByMCPRegistry(ctx, req.Id)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to get MCP registry stats")
	}

	return &v1.MCPRegistryStatsRes{
		TotalCalls:   stats.TotalCalls,
		SuccessCalls: stats.SuccessCalls,
		FailedCalls:  stats.FailedCalls,
		AvgDuration:  float32(stats.AvgDuration),
	}, nil
}
