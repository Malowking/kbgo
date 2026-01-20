package claude_skill

import (
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/agent_tools/framework"
	"github.com/Malowking/kbgo/core/schema"
	"github.com/gogf/gf/v2/frame/g"
)

// ClaudeSkillExecutor Claude Skill 执行器
type ClaudeSkillExecutor struct {
	// skillManager Skill 管理器
	skillManager *SkillManager
}

// NewClaudeSkillExecutor 创建 Claude Skill 执行器
func NewClaudeSkillExecutor(skillManager *SkillManager) *ClaudeSkillExecutor {
	return &ClaudeSkillExecutor{
		skillManager: skillManager,
	}
}

// Execute 执行 Claude Skill
func (e *ClaudeSkillExecutor) Execute(
	ctx framework.ToolContext,
	tool framework.ToolDefinition,
	input map[string]interface{},
) (*schema.ToolResult, error) {
	startTime := time.Now()

	// 提取 skill 名称（去掉 "skill_" 前缀）
	skillToolName := strings.TrimPrefix(tool.Name, "skill_")

	g.Log().Infof(ctx.Context, "[Claude Skill执行器] 执行 %s", skillToolName)

	// 获取 Skill
	skill, exists := e.skillManager.GetSkill(skillToolName)
	if !exists {
		return schema.NewToolResultFromError(
			schema.NewNotFoundError(fmt.Sprintf("Skill 不存在: %s", skillToolName), ""),
		), nil
	}

	// 执行 Skill（不带进度回调，因为框架层不需要）
	result, err := e.skillManager.Executor.ExecuteSkill(ctx.Context, skill, input, nil)
	if err != nil {
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("Skill执行失败: %v", err), ""),
		), nil
	}

	// 检查执行结果
	if !result.Success {
		return schema.NewToolResultFromError(
			schema.NewExecutionError(fmt.Sprintf("Skill执行失败: %s", result.Error), ""),
		), nil
	}

	// 转换为 ToolResult
	toolResult := adaptSkillResultToToolResult(result, skillToolName, time.Since(startTime))

	return toolResult, nil
}

// adaptSkillResultToToolResult 将 Skill 结果转换为 ToolResult
func adaptSkillResultToToolResult(
	skillResult *SkillResult,
	skillName string,
	duration time.Duration,
) *schema.ToolResult {
	// 创建成功的 ToolResult
	result := schema.NewToolResult(skillResult.Output)

	// 添加元数据
	result.WithMetadata("source", "claude_skill")
	result.WithMetadata("skill_name", skillName)

	// 添加执行指标
	result.WithMetrics(schema.NewExecutionMetrics(duration, 0, 0))

	return result
}
