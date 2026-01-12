package skill

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/gogf/gf/v2/os/gctx"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/agent_tools/claude_skills"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

// CreateSkill 创建 Skill
func CreateSkill(ctx context.Context, req *v1.SkillCreateReq, ownerID string) (string, error) {
	// 1. 检查名称是否已存在
	exists, err := dao.ClaudeSkill.Exists(ctx, req.Name, ownerID)
	if err != nil {
		return "", err
	}
	if exists {
		return "", errors.Newf(errors.ErrAlreadyExists, "Skill 名称已存在: %s", req.Name)
	}

	// 2. 计算脚本哈希
	scriptHash := computeScriptHash(req.Script)

	// 3. 序列化 JSON 字段
	requirementsJSON, _ := json.Marshal(req.Requirements)
	toolParametersJSON, _ := json.Marshal(req.ToolParameters)
	metadataJSON, _ := json.Marshal(req.Metadata)

	// 4. 创建 Skill 模型
	id := strings.ReplaceAll(uuid.New().String(), "-", "")
	skill := &gormModel.ClaudeSkill{
		ID:              id,
		Name:            req.Name,
		Description:     req.Description,
		Version:         req.Version,
		Author:          req.Author,
		Category:        req.Category,
		Tags:            req.Tags,
		RuntimeType:     req.RuntimeType,
		RuntimeVersion:  req.RuntimeVersion,
		Requirements:    string(requirementsJSON),
		ToolName:        req.ToolName,
		ToolDescription: req.ToolDescription,
		ToolParameters:  string(toolParametersJSON),
		Script:          req.Script,
		ScriptHash:      scriptHash,
		Metadata:        string(metadataJSON),
		Status:          1, // 默认启用
		IsPublic:        req.IsPublic,
		OwnerID:         ownerID,
	}

	// 5. 保存到数据库
	if err := dao.ClaudeSkill.Create(ctx, skill); err != nil {
		return "", err
	}

	g.Log().Infof(ctx, "Skill 创建成功: ID=%s, Name=%s", id, req.Name)
	return id, nil
}

// UpdateSkill 更新 Skill
func UpdateSkill(ctx context.Context, req *v1.SkillUpdateReq, ownerID string) error {
	// 1. 查询 Skill
	skill, err := dao.ClaudeSkill.GetByID(ctx, req.Id)
	if err != nil {
		return errors.Newf(errors.ErrNotFound, "Skill 不存在: %s", req.Id)
	}

	// 2. 检查权限
	if skill.OwnerID != ownerID {
		return errors.Newf(errors.ErrUnauthorized, "无权限修改此 Skill")
	}

	// 3. 更新字段
	if req.Name != nil {
		// 检查名称是否重复
		exists, err := dao.ClaudeSkill.Exists(ctx, *req.Name, ownerID, req.Id)
		if err != nil {
			return err
		}
		if exists {
			return errors.Newf(errors.ErrInvalidParameter, "Skill 名称已存在: %s", *req.Name)
		}
		skill.Name = *req.Name
	}
	if req.Description != nil {
		skill.Description = *req.Description
	}
	if req.Version != nil {
		skill.Version = *req.Version
	}
	if req.Author != nil {
		skill.Author = *req.Author
	}
	if req.Category != nil {
		skill.Category = *req.Category
	}
	if req.Tags != nil {
		skill.Tags = *req.Tags
	}
	if req.RuntimeType != nil {
		skill.RuntimeType = *req.RuntimeType
	}
	if req.RuntimeVersion != nil {
		skill.RuntimeVersion = *req.RuntimeVersion
	}
	if req.Requirements != nil {
		requirementsJSON, _ := json.Marshal(req.Requirements)
		skill.Requirements = string(requirementsJSON)
	}
	if req.ToolName != nil {
		skill.ToolName = *req.ToolName
	}
	if req.ToolDescription != nil {
		skill.ToolDescription = *req.ToolDescription
	}
	if req.ToolParameters != nil {
		toolParametersJSON, _ := json.Marshal(req.ToolParameters)
		skill.ToolParameters = string(toolParametersJSON)
	}
	if req.Script != nil {
		skill.Script = *req.Script
		skill.ScriptHash = computeScriptHash(*req.Script)
	}
	if req.Status != nil {
		skill.Status = *req.Status
	}
	if req.IsPublic != nil {
		skill.IsPublic = *req.IsPublic
	}
	if req.Metadata != nil {
		metadataJSON, _ := json.Marshal(req.Metadata)
		skill.Metadata = string(metadataJSON)
	}

	// 4. 保存更新
	if err := dao.ClaudeSkill.Update(ctx, skill); err != nil {
		return err
	}

	g.Log().Infof(ctx, "Skill 更新成功: ID=%s", req.Id)
	return nil
}

// DeleteSkill 删除 Skill
func DeleteSkill(ctx context.Context, id string, ownerID string) error {
	// 1. 查询 Skill
	skill, err := dao.ClaudeSkill.GetByID(ctx, id)
	if err != nil {
		return errors.Newf(errors.ErrNotFound, "Skill 不存在: %s", id)
	}

	// 2. 检查权限
	if skill.OwnerID != ownerID {
		return errors.Newf(errors.ErrUnauthorized, "无权限删除此 Skill")
	}

	// 3. 删除
	if err := dao.ClaudeSkill.Delete(ctx, id); err != nil {
		return err
	}

	g.Log().Infof(ctx, "Skill 删除成功: ID=%s", id)
	return nil
}

// GetSkill 获取 Skill 详情
func GetSkill(ctx context.Context, id string, ownerID string) (*v1.SkillGetOneRes, error) {
	// 1. 查询 Skill
	skill, err := dao.ClaudeSkill.GetByID(ctx, id)
	if err != nil {
		return nil, errors.Newf(errors.ErrNotFound, "Skill 不存在: %s", id)
	}

	// 2. 检查权限（只能查看自己的或公开的）
	if skill.OwnerID != ownerID && !skill.IsPublic {
		return nil, errors.Newf(errors.ErrUnauthorized, "无权限查看此 Skill")
	}

	// 3. 转换为响应
	return convertSkillToResponse(skill), nil
}

// ListSkills 获取 Skill 列表
func ListSkills(ctx context.Context, req *v1.SkillGetListReq, ownerID string) (*v1.SkillGetListRes, error) {
	// 构建查询请求
	daoReq := &dao.ListSkillsReq{
		Status:        req.Status,
		Category:      req.Category,
		OwnerID:       ownerID,
		IncludePublic: req.IncludePublic,
		PublicOnly:    req.PublicOnly,
		Keyword:       req.Keyword,
		OrderBy:       req.OrderBy,
		Page:          req.Page,
		PageSize:      req.PageSize,
	}

	// 查询列表
	skills, total, err := dao.ClaudeSkill.List(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	// 转换为响应
	items := make([]*v1.SkillItem, 0, len(skills))
	for _, skill := range skills {
		items = append(items, convertSkillToListItem(skill))
	}

	return &v1.SkillGetListRes{
		List:  items,
		Total: total,
		Page:  req.Page,
	}, nil
}

// ExecuteSkill 执行 Skill
func ExecuteSkill(ctx context.Context, req *v1.SkillExecuteReq, ownerID string, convID string, messageID string) (*v1.SkillExecuteRes, error) {
	// 1. 查询 Skill
	skill, err := dao.ClaudeSkill.GetByID(ctx, req.Id)
	if err != nil {
		return nil, errors.Newf(errors.ErrNotFound, "Skill 不存在: %s", req.Id)
	}

	// 2. 检查权限
	if skill.OwnerID != ownerID && !skill.IsPublic {
		return nil, errors.Newf(errors.ErrUnauthorized, "无权限执行此 Skill")
	}

	// 3. 检查状态
	if skill.Status != 1 {
		return nil, errors.Newf(errors.ErrInvalidParameter, "Skill 未启用")
	}

	// 4. 转换为执行器的 Skill 格式
	execSkill, err := convertToExecutorSkill(skill)
	if err != nil {
		return nil, err
	}

	// 5. 创建执行器并执行
	// 从配置文件读取路径
	venvBaseDir := g.Cfg().MustGet(ctx, "skills.venvBaseDir", "/data/kbgo_venvs").String()
	skillsDir := g.Cfg().MustGet(ctx, "skills.scriptsDir", "/data/kbgo_skills").String()

	executor, err := claude_skills.NewSkillExecutor(venvBaseDir, skillsDir)
	if err != nil {
		return nil, errors.Newf(errors.ErrInternalError, "创建执行器失败: %v", err)
	}

	startTime := time.Now()
	result, err := executor.ExecuteSkill(ctx, execSkill, req.Arguments, nil)
	duration := time.Since(startTime).Milliseconds()

	// 6. 记录调用日志
	logID := strings.ReplaceAll(uuid.New().String(), "-", "")
	reqPayload, _ := json.Marshal(req.Arguments)
	respPayload, _ := json.Marshal(result)

	callLog := &gormModel.ClaudeSkillCallLog{
		ID:              logID,
		SkillID:         req.Id,
		SkillName:       skill.Name,
		ConversationID:  convID,
		MessageID:       messageID,
		RequestPayload:  string(reqPayload),
		ResponsePayload: string(respPayload),
		Success:         err == nil && result.Success,
		ErrorMessage:    "",
		Duration:        duration,
		VenvHash:        "", // TODO: 从执行器获取
		VenvCacheHit:    false,
	}

	if err != nil {
		callLog.ErrorMessage = err.Error()
	} else if !result.Success {
		callLog.ErrorMessage = result.Error
	}

	// 异步保存日志
	go func() {
		if logErr := dao.ClaudeSkillCallLog.Create(gctx.New(), callLog); logErr != nil {
			g.Log().Errorf(ctx, "保存 Skill 调用日志失败: %v", logErr)
		}
	}()

	// 7. 更新统计信息
	go func() {
		if statsErr := dao.ClaudeSkill.UpdateStats(gctx.New(), req.Id, callLog.Success, duration); statsErr != nil {
			g.Log().Errorf(ctx, "更新 Skill 统计失败: %v", statsErr)
		}
	}()

	// 8. 返回结果
	if err != nil {
		return &v1.SkillExecuteRes{
			Success:  false,
			Error:    err.Error(),
			Duration: duration,
		}, nil
	}

	return &v1.SkillExecuteRes{
		Success:  result.Success,
		Output:   result.Output,
		Error:    result.Error,
		Duration: duration,
	}, nil
}

// GetSkillCallLogs 获取 Skill 调用日志
func GetSkillCallLogs(ctx context.Context, req *v1.SkillCallLogsReq, ownerID string) (*v1.SkillCallLogsRes, error) {
	// 1. 查询 Skill（检查权限）
	skill, err := dao.ClaudeSkill.GetByID(ctx, req.Id)
	if err != nil {
		return nil, errors.Newf(errors.ErrNotFound, "Skill 不存在: %s", req.Id)
	}

	if skill.OwnerID != ownerID {
		return nil, errors.Newf(errors.ErrUnauthorized, "无权限查看此 Skill 的日志")
	}

	// 2. 查询日志
	daoReq := &dao.ListSkillCallLogsReq{
		SkillID:        req.Id,
		ConversationID: req.ConversationID,
		Success:        req.Success,
		Page:           req.Page,
		PageSize:       req.PageSize,
	}

	logs, total, err := dao.ClaudeSkillCallLog.List(ctx, daoReq)
	if err != nil {
		return nil, err
	}

	// 3. 转换为响应
	items := make([]*v1.SkillCallLogItem, 0, len(logs))
	for _, log := range logs {
		items = append(items, convertLogToItem(log))
	}

	return &v1.SkillCallLogsRes{
		List:  items,
		Total: total,
		Page:  req.Page,
	}, nil
}

// 辅助函数

func computeScriptHash(script string) string {
	hash := md5.Sum([]byte(script))
	return hex.EncodeToString(hash[:])
}

func convertSkillToResponse(skill *gormModel.ClaudeSkill) *v1.SkillGetOneRes {
	var requirements []string
	json.Unmarshal([]byte(skill.Requirements), &requirements)

	var toolParameters map[string]v1.SkillToolParameterDef
	json.Unmarshal([]byte(skill.ToolParameters), &toolParameters)

	var metadata map[string]interface{}
	json.Unmarshal([]byte(skill.Metadata), &metadata)

	res := &v1.SkillGetOneRes{
		Id:              skill.ID,
		Name:            skill.Name,
		Description:     skill.Description,
		Version:         skill.Version,
		Author:          skill.Author,
		Category:        skill.Category,
		Tags:            skill.Tags,
		RuntimeType:     skill.RuntimeType,
		RuntimeVersion:  skill.RuntimeVersion,
		Requirements:    requirements,
		ToolName:        skill.ToolName,
		ToolDescription: skill.ToolDescription,
		ToolParameters:  toolParameters,
		Script:          skill.Script,
		ScriptHash:      skill.ScriptHash,
		Metadata:        metadata,
		CallCount:       skill.CallCount,
		SuccessCount:    skill.SuccessCount,
		FailCount:       skill.FailCount,
		AvgDuration:     skill.AvgDuration,
		Status:          skill.Status,
		IsPublic:        skill.IsPublic,
		OwnerID:         skill.OwnerID,
	}

	if skill.LastUsedAt != nil {
		res.LastUsedAt = skill.LastUsedAt.Format("2006-01-02 15:04:05")
	}
	if skill.CreateTime != nil {
		res.CreateTime = skill.CreateTime.Format("2006-01-02 15:04:05")
	}
	if skill.UpdateTime != nil {
		res.UpdateTime = skill.UpdateTime.Format("2006-01-02 15:04:05")
	}

	return res
}

func convertSkillToListItem(skill *gormModel.ClaudeSkill) *v1.SkillItem {
	var requirements []string
	json.Unmarshal([]byte(skill.Requirements), &requirements)

	item := &v1.SkillItem{
		Id:              skill.ID,
		Name:            skill.Name,
		Description:     skill.Description,
		Version:         skill.Version,
		Author:          skill.Author,
		Category:        skill.Category,
		Tags:            skill.Tags,
		RuntimeType:     skill.RuntimeType,
		Requirements:    requirements,
		ToolName:        skill.ToolName,
		ToolDescription: skill.ToolDescription,
		CallCount:       skill.CallCount,
		SuccessCount:    skill.SuccessCount,
		AvgDuration:     skill.AvgDuration,
		Status:          skill.Status,
		IsPublic:        skill.IsPublic,
		OwnerID:         skill.OwnerID,
	}

	if skill.LastUsedAt != nil {
		item.LastUsedAt = skill.LastUsedAt.Format("2006-01-02 15:04:05")
	}
	if skill.CreateTime != nil {
		item.CreateTime = skill.CreateTime.Format("2006-01-02 15:04:05")
	}
	if skill.UpdateTime != nil {
		item.UpdateTime = skill.UpdateTime.Format("2006-01-02 15:04:05")
	}

	return item
}

func convertToExecutorSkill(skill *gormModel.ClaudeSkill) (*claude_skills.Skill, error) {
	var requirements []string
	if err := json.Unmarshal([]byte(skill.Requirements), &requirements); err != nil {
		return nil, errors.Newf(errors.ErrInternalError, "解析依赖列表失败: %v", err)
	}

	var toolParameters map[string]claude_skills.SkillToolParameter
	if err := json.Unmarshal([]byte(skill.ToolParameters), &toolParameters); err != nil {
		return nil, errors.Newf(errors.ErrInternalError, "解析工具参数失败: %v", err)
	}

	var metadata map[string]interface{}
	json.Unmarshal([]byte(skill.Metadata), &metadata)

	return &claude_skills.Skill{
		ID:          skill.ID,
		Name:        skill.Name,
		Description: skill.Description,
		Version:     skill.Version,
		Runtime: claude_skills.SkillRuntime{
			Type:         skill.RuntimeType,
			Version:      skill.RuntimeVersion,
			Requirements: requirements,
		},
		Tool: claude_skills.SkillTool{
			Name:        skill.ToolName,
			Description: skill.ToolDescription,
			Parameters:  toolParameters,
		},
		Script:   skill.Script,
		Metadata: metadata,
	}, nil
}

func convertLogToItem(log *gormModel.ClaudeSkillCallLog) *v1.SkillCallLogItem {
	item := &v1.SkillCallLogItem{
		Id:              log.ID,
		SkillID:         log.SkillID,
		SkillName:       log.SkillName,
		ConversationID:  log.ConversationID,
		MessageID:       log.MessageID,
		RequestPayload:  log.RequestPayload,
		ResponsePayload: log.ResponsePayload,
		Success:         log.Success,
		ErrorMessage:    log.ErrorMessage,
		Duration:        log.Duration,
		VenvHash:        log.VenvHash,
		VenvCacheHit:    log.VenvCacheHit,
	}

	if log.CreateTime != nil {
		item.CreateTime = log.CreateTime.Format("2006-01-02 15:04:05")
	}

	return item
}
