package kbgo

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/logic/skill"
	"github.com/gogf/gf/v2/frame/g"
)

// SkillCreate 创建 Skill
func (c *ControllerV1) SkillCreate(ctx context.Context, req *v1.SkillCreateReq) (res *v1.SkillCreateRes, err error) {
	g.Log().Infof(ctx, "SkillCreate request received - Name: %s, RuntimeType: %s, ToolName: %s",
		req.Name, req.RuntimeType, req.ToolName)

	// TODO: 从上下文中获取用户ID
	ownerID := "default_user" // 临时使用默认用户ID

	id, err := skill.CreateSkill(ctx, req, ownerID)
	if err != nil {
		return nil, err
	}

	return &v1.SkillCreateRes{Id: id}, nil
}

// SkillUpdate 更新 Skill
func (c *ControllerV1) SkillUpdate(ctx context.Context, req *v1.SkillUpdateReq) (res *v1.SkillUpdateRes, err error) {
	g.Log().Infof(ctx, "SkillUpdate request received - Id: %s", req.Id)

	// TODO: 从上下文中获取用户ID
	ownerID := "default_user"

	if err := skill.UpdateSkill(ctx, req, ownerID); err != nil {
		return nil, err
	}

	return &v1.SkillUpdateRes{}, nil
}

// SkillDelete 删除 Skill
func (c *ControllerV1) SkillDelete(ctx context.Context, req *v1.SkillDeleteReq) (res *v1.SkillDeleteRes, err error) {
	g.Log().Infof(ctx, "SkillDelete request received - Id: %s", req.Id)

	// TODO: 从上下文中获取用户ID
	ownerID := "default_user"

	if err := skill.DeleteSkill(ctx, req.Id, ownerID); err != nil {
		return nil, err
	}

	return &v1.SkillDeleteRes{}, nil
}

// SkillGetOne 获取单个 Skill
func (c *ControllerV1) SkillGetOne(ctx context.Context, req *v1.SkillGetOneReq) (res *v1.SkillGetOneRes, err error) {
	g.Log().Infof(ctx, "SkillGetOne request received - Id: %s", req.Id)

	// TODO: 从上下文中获取用户ID
	ownerID := "default_user"

	return skill.GetSkill(ctx, req.Id, ownerID)
}

// SkillGetList 获取 Skill 列表
func (c *ControllerV1) SkillGetList(ctx context.Context, req *v1.SkillGetListReq) (res *v1.SkillGetListRes, err error) {
	g.Log().Infof(ctx, "SkillGetList request received - Page: %d, PageSize: %d, Category: %s, Keyword: %s",
		req.Page, req.PageSize, req.Category, req.Keyword)

	// TODO: 从上下文中获取用户ID
	ownerID := "default_user"

	return skill.ListSkills(ctx, req, ownerID)
}

// SkillExecute 执行 Skill
func (c *ControllerV1) SkillExecute(ctx context.Context, req *v1.SkillExecuteReq) (res *v1.SkillExecuteRes, err error) {
	g.Log().Infof(ctx, "SkillExecute request received - Id: %s", req.Id)

	// TODO: 从上下文中获取用户ID、会话ID、消息ID
	ownerID := "default_user"
	convID := ""
	messageID := ""

	return skill.ExecuteSkill(ctx, req, ownerID, convID, messageID)
}

// SkillCallLogs 获取 Skill 调用日志
func (c *ControllerV1) SkillCallLogs(ctx context.Context, req *v1.SkillCallLogsReq) (res *v1.SkillCallLogsRes, err error) {
	g.Log().Infof(ctx, "SkillCallLogs request received - Id: %s, Page: %d, PageSize: %d",
		req.Id, req.Page, req.PageSize)

	// TODO: 从上下文中获取用户ID
	ownerID := "default_user"

	return skill.GetSkillCallLogs(ctx, req, ownerID)
}

// SkillCategories 获取 Skill 分类列表
func (c *ControllerV1) SkillCategories(ctx context.Context, req *v1.SkillCategoriesReq) (res *v1.SkillCategoriesRes, err error) {
	g.Log().Infof(ctx, "SkillCategories request received")

	// TODO: 实现分类统计逻辑
	// 这里返回一些常见的分类
	categories := []v1.SkillCategoryItem{
		{Name: "data_analysis", Count: 0},
		{Name: "web_scraping", Count: 0},
		{Name: "file_processing", Count: 0},
		{Name: "api_integration", Count: 0},
		{Name: "automation", Count: 0},
		{Name: "machine_learning", Count: 0},
		{Name: "text_processing", Count: 0},
		{Name: "image_processing", Count: 0},
		{Name: "other", Count: 0},
	}

	return &v1.SkillCategoriesRes{
		Categories: categories,
	}, nil
}
