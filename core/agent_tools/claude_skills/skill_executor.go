package claude_skills

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gogf/gf/v2/frame/g"
)

// ProgressCallback 进度回调函数类型
// stage: 阶段名称 (preparing_venv, creating_venv, installing_deps, preparing_script, executing_script)
// message: 进度消息
// metadata: 额外的元数据
type ProgressCallback func(stage string, message string, metadata map[string]interface{})

// SkillExecutor Skill 执行器
type SkillExecutor struct {
	venvManager *VenvManager
	skillsDir   string // skills 脚本存储目录
}

// NewSkillExecutor 创建 Skill 执行器
func NewSkillExecutor(venvBaseDir, skillsDir string) (*SkillExecutor, error) {
	venvManager, err := NewVenvManager(venvBaseDir)
	if err != nil {
		return nil, err
	}

	// 确保 skills 目录存在
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 skills 目录失败: %w", err)
	}

	return &SkillExecutor{
		venvManager: venvManager,
		skillsDir:   skillsDir,
	}, nil
}

// ExecuteSkill 执行 Skill（带进度回调）
func (e *SkillExecutor) ExecuteSkill(ctx context.Context, skill *Skill, args map[string]interface{}, progressCallback ProgressCallback) (*SkillResult, error) {
	startTime := time.Now()

	g.Log().Infof(ctx, "[Skill执行] 开始执行: %s", skill.Name)

	// 验证参数
	if err := skill.ValidateArgs(args); err != nil {
		return nil, fmt.Errorf("参数验证失败: %w", err)
	}

	// 发送准备虚拟环境进度
	if progressCallback != nil {
		progressCallback("preparing_venv", "准备虚拟环境...", map[string]interface{}{
			"requirements_count": len(skill.Runtime.Requirements),
		})
	}

	// 1. 获取或创建虚拟环境（带进度回调）
	venv, err := e.venvManager.GetOrCreateVenv(ctx, skill.Runtime.Requirements, progressCallback)
	if err != nil {
		return nil, fmt.Errorf("准备虚拟环境失败: %w", err)
	}

	// 发送准备脚本进度
	if progressCallback != nil {
		progressCallback("preparing_script", "准备执行脚本...", nil)
	}

	// 2. 准备脚本文件
	scriptPath, err := e.prepareScript(skill)
	if err != nil {
		return nil, fmt.Errorf("准备脚本失败: %w", err)
	}

	// 发送执行脚本进度
	if progressCallback != nil {
		progressCallback("executing_script", "执行脚本中...", map[string]interface{}{
			"script_path": scriptPath,
		})
	}

	// 3. 执行脚本
	output, err := e.runScript(ctx, venv.PythonPath, scriptPath, args)
	if err != nil {
		return &SkillResult{
			Success:  false,
			Error:    err.Error(),
			Duration: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 4. 解析输出
	result := &SkillResult{
		Success:  true,
		Output:   output,
		Duration: time.Since(startTime).Milliseconds(),
	}

	g.Log().Infof(ctx, "[Skill执行] 完成: %s (耗时: %dms)", skill.Name, result.Duration)

	return result, nil
}

// prepareScript 准备脚本文件
func (e *SkillExecutor) prepareScript(skill *Skill) (string, error) {
	// 使用 skill ID 作为文件名
	scriptPath := filepath.Join(e.skillsDir, fmt.Sprintf("%s.py", skill.ID))

	// 如果脚本已存在且内容相同，直接返回
	if existingContent, err := os.ReadFile(scriptPath); err == nil {
		if string(existingContent) == skill.Script {
			return scriptPath, nil
		}
	}

	// 写入脚本文件
	if err := os.WriteFile(scriptPath, []byte(skill.Script), 0644); err != nil {
		return "", fmt.Errorf("写入脚本文件失败: %w", err)
	}

	return scriptPath, nil
}

// runScript 运行脚本
func (e *SkillExecutor) runScript(ctx context.Context, pythonPath, scriptPath string, args map[string]interface{}) (string, error) {
	// 将参数序列化为 JSON
	argsJSON, err := json.Marshal(args)
	if err != nil {
		return "", fmt.Errorf("序列化参数失败: %w", err)
	}

	// 执行脚本
	cmd := exec.CommandContext(ctx, pythonPath, scriptPath, string(argsJSON))

	// 设置超时（可配置）
	timeout := 5 * time.Minute
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd = exec.CommandContext(ctx, pythonPath, scriptPath, string(argsJSON))

	// 捕获输出
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("脚本执行失败: %w, output: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}

// Skill Skill 定义
type Skill struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Version     string                 `json:"version"`
	Runtime     SkillRuntime           `json:"runtime"`
	Tool        SkillTool              `json:"tool"`
	Script      string                 `json:"script"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// SkillRuntime 运行时配置
type SkillRuntime struct {
	Type         string   `json:"type"`         // python, node, shell
	Version      string   `json:"version"`      // 3.9+, 18+
	Requirements []string `json:"requirements"` // 依赖列表
}

// SkillTool 工具定义
type SkillTool struct {
	Name        string                        `json:"name"`
	Description string                        `json:"description"`
	Parameters  map[string]SkillToolParameter `json:"parameters"`
}

// SkillToolParameter 工具参数
type SkillToolParameter struct {
	Type        string      `json:"type"` // string, number, boolean, array, object
	Required    bool        `json:"required"`
	Description string      `json:"description"`
	Default     interface{} `json:"default,omitempty"`
}

// SkillResult 执行结果
type SkillResult struct {
	Success  bool   `json:"success"`
	Output   string `json:"output"`
	Error    string `json:"error,omitempty"`
	Duration int64  `json:"duration"` // 毫秒
}

// ValidateArgs 验证参数
func (s *Skill) ValidateArgs(args map[string]interface{}) error {
	for paramName, paramDef := range s.Tool.Parameters {
		value, exists := args[paramName]

		// 检查必需参数
		if paramDef.Required && !exists {
			return fmt.Errorf("缺少必需参数: %s", paramName)
		}

		// 如果参数不存在但有默认值，使用默认值
		if !exists && paramDef.Default != nil {
			args[paramName] = paramDef.Default
			continue
		}

		// 类型检查（简单实现）
		if exists {
			if err := validateType(value, paramDef.Type); err != nil {
				return fmt.Errorf("参数 %s 类型错误: %w", paramName, err)
			}
		}
	}

	return nil
}

// validateType 验证类型
func validateType(value interface{}, expectedType string) error {
	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("期望 string，实际为 %T", value)
		}
	case "number":
		switch value.(type) {
		case int, int64, float64, float32:
			return nil
		default:
			return fmt.Errorf("期望 number，实际为 %T", value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("期望 boolean，实际为 %T", value)
		}
	case "array":
		// 使用反射检查是否为切片或数组类型
		v := fmt.Sprintf("%T", value)
		if !strings.HasPrefix(v, "[]") {
			return fmt.Errorf("期望 array，实际为 %T", value)
		}
	case "object":
		if _, ok := value.(map[string]interface{}); !ok {
			return fmt.Errorf("期望 object，实际为 %T", value)
		}
	}

	return nil
}
