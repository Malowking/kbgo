package claude_skills

import (
	"context"
	"fmt"
	"time"

	"github.com/Malowking/kbgo/pkg/schema"
)

// SkillManager Skill 管理器
type SkillManager struct {
	Executor *SkillExecutor
	skills   map[string]*Skill // skillID -> Skill
}

// NewSkillManager 创建 Skill 管理器
func NewSkillManager(venvBaseDir, skillsDir string) (*SkillManager, error) {
	executor, err := NewSkillExecutor(venvBaseDir, skillsDir)
	if err != nil {
		return nil, err
	}

	return &SkillManager{
		Executor: executor,
		skills:   make(map[string]*Skill),
	}, nil
}

// RegisterSkill 注册 Skill
func (m *SkillManager) RegisterSkill(skill *Skill) error {
	// 验证 Skill 定义
	if skill.ID == "" {
		return fmt.Errorf("skill ID 不能为空")
	}
	if skill.Name == "" {
		return fmt.Errorf("skill 名称不能为空")
	}
	if skill.Script == "" {
		return fmt.Errorf("skill 脚本不能为空")
	}

	m.skills[skill.ID] = skill
	return nil
}

// UnregisterSkill 注销 Skill
func (m *SkillManager) UnregisterSkill(skillID string) {
	delete(m.skills, skillID)
}

// GetSkill 获取 Skill
func (m *SkillManager) GetSkill(skillID string) (*Skill, bool) {
	skill, exists := m.skills[skillID]
	return skill, exists
}

// GetAllSkills 获取所有 Skills
func (m *SkillManager) GetAllSkills() []*Skill {
	skills := make([]*Skill, 0, len(m.skills))
	for _, skill := range m.skills {
		skills = append(skills, skill)
	}
	return skills
}

// GetLLMTools 获取 LLM 工具定义
func (m *SkillManager) GetLLMTools() []*schema.ToolInfo {
	tools := make([]*schema.ToolInfo, 0, len(m.skills))

	for _, skill := range m.skills {
		toolInfo := m.convertSkillToToolInfo(skill)
		tools = append(tools, toolInfo)
	}

	return tools
}

// convertSkillToToolInfo 将 Skill 转换为 ToolInfo
func (m *SkillManager) convertSkillToToolInfo(skill *Skill) *schema.ToolInfo {
	// 构建参数定义
	params := make(map[string]*schema.ParameterInfo)
	for paramName, paramDef := range skill.Tool.Parameters {
		params[paramName] = &schema.ParameterInfo{
			Type:     paramDef.Type,
			Desc:     paramDef.Description,
			Required: paramDef.Required,
		}
	}

	// 使用 skill__前缀避免与其他工具冲突
	toolName := fmt.Sprintf("skill__%s", skill.Tool.Name)

	return &schema.ToolInfo{
		Name:        toolName,
		Desc:        skill.Tool.Description,
		ParamsOneOf: schema.NewParamsOneOfByParams(params),
	}
}

// CleanupOldVenvs 清理旧的虚拟环境
func (m *SkillManager) CleanupOldVenvs(ctx context.Context) error {
	// 清理 7 天未使用的虚拟环境
	return m.Executor.venvManager.CleanupOldVenvs(ctx, 7*24*time.Hour)
}
