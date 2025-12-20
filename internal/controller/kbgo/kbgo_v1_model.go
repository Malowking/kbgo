package kbgo

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/model"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

// ReloadModels 重新加载模型配置（热更新）
func (c *ControllerV1) ReloadModels(ctx context.Context, req *v1.ReloadModelsReq) (res *v1.ReloadModelsRes, err error) {
	g.Log().Info(ctx, "ReloadModels request received")

	// 获取数据库连接
	db := dao.GetDB()

	// 重新加载模型配置
	if err := model.Registry.Reload(ctx, db); err != nil {
		g.Log().Errorf(ctx, "Failed to reload models: %v", err)
		return nil, err
	}

	return &v1.ReloadModelsRes{
		Success: true,
		Message: "Model registry reload success",
		Count:   model.Registry.Count(),
	}, nil
}

// ListModels 列出所有模型
func (c *ControllerV1) ListModels(ctx context.Context, req *v1.ListModelsReq) (res *v1.ListModelsRes, err error) {
	g.Log().Info(ctx, "ListModels request received")

	// 根据类型过滤（可选）
	var models []*model.ModelConfig
	if req.ModelType != "" {
		models = model.Registry.GetByType(model.ModelType(req.ModelType))
	} else {
		models = model.Registry.List()
	}

	return &v1.ListModelsRes{
		Models: models,
		Count:  len(models),
	}, nil
}

// GetModel 获取单个模型详情
func (c *ControllerV1) GetModel(ctx context.Context, req *v1.GetModelReq) (res *v1.GetModelRes, err error) {
	g.Log().Infof(ctx, "GetModel request received - ModelID: %s", req.ModelID)

	mc := model.Registry.Get(req.ModelID)
	if mc == nil {
		g.Log().Errorf(ctx, "Model not found: %s", req.ModelID)
		return nil, gerror.Newf("Model not found: %s", req.ModelID)
	}

	return &v1.GetModelRes{
		Model: mc,
	}, nil
}

// RegisterModel 注册新模型
func (c *ControllerV1) RegisterModel(ctx context.Context, req *v1.RegisterModelReq) (res *v1.RegisterModelRes, err error) {
	g.Log().Infof(ctx, "RegisterModel request received - ModelName: %s, ModelType: %s", req.ModelName, req.ModelType)

	// 构建 Extra JSON 字段
	extra := make(map[string]interface{})
	if req.MaxCompletionTokens > 0 {
		extra["max_completion_tokens"] = req.MaxCompletionTokens
	}
	if req.Dimension > 0 {
		extra["dimension"] = req.Dimension
	}
	if req.Config != nil {
		for k, v := range req.Config {
			extra[k] = v
		}
	}

	// 序列化 Extra 为 JSON 字符串
	// 注意：MySQL JSON字段不接受空字符串，至少要是空对象 {}
	extraJSON := "{}"
	if len(extra) > 0 {
		extraBytes, err := gjson.Marshal(extra)
		if err != nil {
			g.Log().Errorf(ctx, "Failed to marshal extra config: %v", err)
			return nil, gerror.Newf("Failed to marshal extra config: %v", err)
		}
		extraJSON = string(extraBytes)
	}

	// 创建模型记录（ModelID将由BeforeCreate钩子自动生成）
	aiModel := &gormModel.AIModel{
		ModelType: req.ModelType,
		Provider:  req.Provider,
		ModelName: req.ModelName,
		BaseURL:   req.BaseURL,
		APIKey:    req.APIKey,
		Extra:     extraJSON,
		Enabled:   req.Enabled,
	}

	// 保存到数据库
	if err := dao.AIModel.Create(ctx, aiModel); err != nil {
		g.Log().Errorf(ctx, "Failed to create model: %v", err)
		return nil, gerror.Newf("Failed to create model: %v", err)
	}

	// 重新加载模型注册表
	db := dao.GetDB()
	if err := model.Registry.Reload(ctx, db); err != nil {
		g.Log().Errorf(ctx, "Failed to reload model registry: %v", err)
		// 虽然重载失败，但模型已创建成功，返回警告
		return &v1.RegisterModelRes{
			Success: true,
			Message: "Model registered successfully, but failed to reload registry. Please call /v1/model/reload manually.",
			ModelID: aiModel.ModelID,
		}, nil
	}

	g.Log().Infof(ctx, "Model registered successfully with ID: %s", aiModel.ModelID)
	return &v1.RegisterModelRes{
		Success: true,
		Message: "Model registered and loaded successfully",
		ModelID: aiModel.ModelID,
	}, nil
}

// UpdateModel 更新模型配置
func (c *ControllerV1) UpdateModel(ctx context.Context, req *v1.UpdateModelReq) (res *v1.UpdateModelRes, err error) {
	g.Log().Infof(ctx, "UpdateModel request received - ModelID: %s", req.ModelID)

	// 检查模型是否存在
	existingModel, err := dao.AIModel.GetByID(ctx, req.ModelID)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to get model: %v", err)
		return nil, gerror.Newf("Failed to get model: %v", err)
	}
	if existingModel == nil {
		g.Log().Errorf(ctx, "Model not found: %s", req.ModelID)
		return nil, gerror.Newf("Model not found: %s", req.ModelID)
	}

	// 只更新传入的字段（使用指针判断是否传值）
	if req.ModelName != nil {
		existingModel.ModelName = *req.ModelName
	}
	if req.ModelType != nil {
		existingModel.ModelType = *req.ModelType
	}
	if req.Provider != nil {
		existingModel.Provider = *req.Provider
	}
	if req.Version != nil {
		existingModel.Version = *req.Version
	}
	if req.BaseURL != nil {
		existingModel.BaseURL = *req.BaseURL
	}
	if req.APIKey != nil {
		existingModel.APIKey = *req.APIKey
	}
	if req.Enabled != nil {
		existingModel.Enabled = *req.Enabled
	}
	if req.Extra != nil {
		// 验证 Extra 字段是否为有效的 JSON
		var extraTest interface{}
		if err := gjson.Unmarshal([]byte(*req.Extra), &extraTest); err != nil {
			g.Log().Errorf(ctx, "Invalid JSON format for extra field: %v", err)
			return nil, gerror.Newf("Invalid JSON format for extra field: %v", err)
		}
		existingModel.Extra = *req.Extra
	}

	// 保存更新
	if err := dao.AIModel.Update(ctx, existingModel); err != nil {
		g.Log().Errorf(ctx, "Failed to update model: %v", err)
		return nil, gerror.Newf("Failed to update model: %v", err)
	}

	// 重新加载模型注册表
	db := dao.GetDB()
	if err := model.Registry.Reload(ctx, db); err != nil {
		g.Log().Errorf(ctx, "Failed to reload model registry: %v", err)
		return &v1.UpdateModelRes{
			Success: true,
			Message: "Model updated successfully, but failed to reload registry. Please call /v1/model/reload manually.",
		}, nil
	}

	g.Log().Infof(ctx, "Model updated successfully: %s", req.ModelID)
	return &v1.UpdateModelRes{
		Success: true,
		Message: "Model updated and reloaded successfully",
	}, nil
}

// DeleteModel 删除模型
func (c *ControllerV1) DeleteModel(ctx context.Context, req *v1.DeleteModelReq) (res *v1.DeleteModelRes, err error) {
	g.Log().Infof(ctx, "DeleteModel request received - ModelID: %s", req.ModelID)

	// 检查模型是否存在
	existingModel, err := dao.AIModel.GetByID(ctx, req.ModelID)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to get model: %v", err)
		return nil, gerror.Newf("Failed to get model: %v", err)
	}
	if existingModel == nil {
		g.Log().Errorf(ctx, "Model not found: %s", req.ModelID)
		return nil, gerror.Newf("Model not found: %s", req.ModelID)
	}

	// 删除模型
	if err := dao.AIModel.Delete(ctx, req.ModelID); err != nil {
		g.Log().Errorf(ctx, "Failed to delete model: %v", err)
		return nil, gerror.Newf("Failed to delete model: %v", err)
	}

	// 重新加载模型注册表
	db := dao.GetDB()
	if err := model.Registry.Reload(ctx, db); err != nil {
		g.Log().Errorf(ctx, "Failed to reload model registry: %v", err)
		return &v1.DeleteModelRes{
			Success: true,
			Message: "Model deleted successfully, but failed to reload registry. Please call /v1/model/reload manually.",
		}, nil
	}

	g.Log().Infof(ctx, "Model deleted successfully: %s", req.ModelID)
	return &v1.DeleteModelRes{
		Success: true,
		Message: "Model deleted and registry reloaded successfully",
	}, nil
}

// SetRewriteModel 设置重写模型
func (c *ControllerV1) SetRewriteModel(ctx context.Context, req *v1.SetRewriteModelReq) (res *v1.SetRewriteModelRes, err error) {
	g.Log().Infof(ctx, "SetRewriteModel request received - ModelID: %s", req.ModelID)

	db := dao.GetDB()

	// 如果 ModelID 为空，表示取消重写模型
	if req.ModelID == "" {
		// 清空内存中的重写模型
		if err := model.Registry.SetRewriteModel(""); err != nil {
			g.Log().Errorf(ctx, "Failed to clear rewrite model: %v", err)
			return nil, err
		}

		// 从数据库中移除所有模型的 is_rewrite 标记
		// 查询所有有 is_rewrite 标记的模型
		var models []gormModel.AIModel
		if err := db.Find(&models).Error; err != nil {
			g.Log().Errorf(ctx, "Failed to query models: %v", err)
			return nil, gerror.Newf("Failed to query models: %v", err)
		}

		// 逐个移除 is_rewrite 标记
		for _, m := range models {
			if m.Extra == "" || m.Extra == "{}" {
				continue
			}

			var extra map[string]interface{}
			if err := gjson.Unmarshal([]byte(m.Extra), &extra); err != nil {
				continue
			}

			// 检查是否有 is_rewrite 标记
			if _, exists := extra["is_rewrite"]; exists {
				// 删除 is_rewrite 标记
				delete(extra, "is_rewrite")

				// 序列化回 JSON
				extraBytes, err := gjson.Marshal(extra)
				if err != nil {
					g.Log().Warningf(ctx, "Failed to marshal extra for model %s: %v", m.ModelID, err)
					continue
				}

				// 更新数据库
				if err := db.Model(&gormModel.AIModel{}).Where("model_id = ?", m.ModelID).
					Update("extra", string(extraBytes)).Error; err != nil {
					g.Log().Warningf(ctx, "Failed to update model %s: %v", m.ModelID, err)
				}
			}
		}

		// 重新加载模型注册表
		if err := model.Registry.Reload(ctx, db); err != nil {
			g.Log().Errorf(ctx, "Failed to reload model registry: %v", err)
		}

		g.Log().Info(ctx, "Rewrite model cleared successfully")
		return &v1.SetRewriteModelRes{
			Success: true,
			Message: "Rewrite model cleared successfully",
		}, nil
	}

	// 检查模型是否存在
	existingModel, err := dao.AIModel.GetByID(ctx, req.ModelID)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to get model: %v", err)
		return nil, gerror.Newf("Failed to get model: %v", err)
	}
	if existingModel == nil {
		g.Log().Errorf(ctx, "Model not found: %s", req.ModelID)
		return nil, gerror.Newf("Model not found: %s", req.ModelID)
	}

	// 检查模型类型是否为 LLM
	if existingModel.ModelType != "llm" {
		return nil, gerror.New("Rewrite model must be LLM type")
	}

	// 检查模型是否启用
	if !existingModel.Enabled {
		return nil, gerror.New("Cannot set disabled model as rewrite model")
	}

	// 1. 先移除所有模型的 is_rewrite 标记
	var models []gormModel.AIModel
	if err := db.Find(&models).Error; err != nil {
		g.Log().Errorf(ctx, "Failed to query models: %v", err)
		return nil, gerror.Newf("Failed to query models: %v", err)
	}

	for _, m := range models {
		if m.Extra == "" || m.Extra == "{}" {
			continue
		}

		var extra map[string]interface{}
		if err := gjson.Unmarshal([]byte(m.Extra), &extra); err != nil {
			continue
		}

		// 检查是否有 is_rewrite 标记
		if _, exists := extra["is_rewrite"]; exists {
			// 删除 is_rewrite 标记
			delete(extra, "is_rewrite")

			// 序列化回 JSON
			extraBytes, err := gjson.Marshal(extra)
			if err != nil {
				g.Log().Warningf(ctx, "Failed to marshal extra for model %s: %v", m.ModelID, err)
				continue
			}

			// 更新数据库
			if err := db.Model(&gormModel.AIModel{}).Where("model_id = ?", m.ModelID).
				Update("extra", string(extraBytes)).Error; err != nil {
				g.Log().Warningf(ctx, "Failed to update model %s: %v", m.ModelID, err)
			}
		}
	}

	// 2. 为指定模型添加 is_rewrite 标记
	// 解析现有的 extra JSON
	var extra map[string]interface{}
	if existingModel.Extra != "" && existingModel.Extra != "{}" {
		if err := gjson.Unmarshal([]byte(existingModel.Extra), &extra); err != nil {
			g.Log().Warningf(ctx, "Failed to parse existing extra: %v, using empty map", err)
			extra = make(map[string]interface{})
		}
	} else {
		extra = make(map[string]interface{})
	}

	// 添加 is_rewrite 标记
	extra["is_rewrite"] = true

	// 序列化回 JSON
	extraBytes, err := gjson.Marshal(extra)
	if err != nil {
		g.Log().Errorf(ctx, "Failed to marshal extra: %v", err)
		return nil, gerror.Newf("Failed to marshal extra: %v", err)
	}
	existingModel.Extra = string(extraBytes)

	// 更新数据库
	if err := dao.AIModel.Update(ctx, existingModel); err != nil {
		g.Log().Errorf(ctx, "Failed to update model: %v", err)
		return nil, gerror.Newf("Failed to update model: %v", err)
	}

	// 3. 更新内存中的重写模型
	if err := model.Registry.SetRewriteModel(req.ModelID); err != nil {
		g.Log().Errorf(ctx, "Failed to set rewrite model in registry: %v", err)
		return nil, err
	}

	// 4. 重新加载模型注册表（确保数据库和内存一致）
	if err := model.Registry.Reload(ctx, db); err != nil {
		g.Log().Errorf(ctx, "Failed to reload model registry: %v", err)
		return &v1.SetRewriteModelRes{
			Success: true,
			Message: "Rewrite model set successfully, but failed to reload registry. Please call /v1/model/reload manually.",
		}, nil
	}

	g.Log().Infof(ctx, "Rewrite model set successfully: %s", req.ModelID)
	return &v1.SetRewriteModelRes{
		Success: true,
		Message: "Rewrite model set successfully",
	}, nil
}

// GetRewriteModel 获取当前重写模型
func (c *ControllerV1) GetRewriteModel(ctx context.Context, req *v1.GetRewriteModelReq) (res *v1.GetRewriteModelRes, err error) {
	g.Log().Info(ctx, "GetRewriteModel request received")

	rewriteModel := model.Registry.GetRewriteModel()

	return &v1.GetRewriteModelRes{
		RewriteModel: rewriteModel,
		Configured:   rewriteModel != nil,
	}, nil
}
