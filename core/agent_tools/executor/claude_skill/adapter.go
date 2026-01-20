package claude_skill

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/core/schema"
)

// ClaudeSkillAdapter Claude Skill 结果适配器
// 职责：将 Skill 输出转换为统一的 ToolResult
type ClaudeSkillAdapter struct{}

// NewClaudeSkillAdapter 创建 Claude Skill 适配器
func NewClaudeSkillAdapter() *ClaudeSkillAdapter {
	return &ClaudeSkillAdapter{}
}

// ToToolResult 将 Skill 结果转换为 ToolResult
func (a *ClaudeSkillAdapter) ToToolResult(
	skillResult *SkillResult,
	skillName string,
	duration time.Duration,
) (*schema.ToolResult, error) {
	if skillResult == nil {
		return schema.NewToolResultWithError(
			schema.ErrCodeInternal,
			"Skill结果为空",
			"",
		), nil
	}

	// 如果执行失败
	if !skillResult.Success {
		return schema.NewToolResultFromError(
			schema.NewExecutionError("Skill执行失败", skillResult.Error),
		), nil
	}

	// 尝试解析输出（可能是 JSON、文本等）
	data, artifacts := parseSkillOutput(skillResult.Output)

	// 创建成功的 ToolResult
	result := schema.NewToolResult(data)

	// 添加附件
	for _, artifact := range artifacts {
		result.WithArtifact(artifact)
	}

	// 添加元数据
	result.WithMetadata("source", "claude_skill")
	result.WithMetadata("skill_name", skillName)
	result.WithMetadata("skill_duration_ms", skillResult.Duration)

	// 添加执行指标
	result.WithMetrics(schema.NewExecutionMetrics(duration, 0, 0))

	return result, nil
}

// parseSkillOutput 解析 Skill 输出
// 尝试识别 JSON、文件路径等特殊格式
func parseSkillOutput(output string) (data interface{}, artifacts []*schema.Artifact) {
	output = strings.TrimSpace(output)

	// 尝试解析为 JSON
	var jsonData interface{}
	if err := json.Unmarshal([]byte(output), &jsonData); err == nil {
		// 检查是否包含文件信息
		if jsonMap, ok := jsonData.(map[string]interface{}); ok {
			// 检查是否有文件字段
			if fileURL, ok := jsonMap["file_url"].(string); ok {
				artifact := &schema.Artifact{
					Type: "file",
					URL:  fileURL,
				}
				if fileName, ok := jsonMap["file_name"].(string); ok {
					artifact.Name = fileName
				}
				if mimeType, ok := jsonMap["mime_type"].(string); ok {
					artifact.MimeType = mimeType
				}
				if size, ok := jsonMap["file_size"].(float64); ok {
					artifact.Size = int64(size)
				}
				artifacts = append(artifacts, artifact)
			}
		}
		return jsonData, artifacts
	}

	// 检查是否是文件路径（简单判断）
	if strings.HasPrefix(output, "/") || strings.HasPrefix(output, "http://") || strings.HasPrefix(output, "https://") {
		artifact := &schema.Artifact{
			Type: "file",
			URL:  output,
			Name: extractFileName(output),
		}
		artifacts = append(artifacts, artifact)
	}

	// 默认返回原始文本
	return output, artifacts
}

// extractFileName 从路径中提取文件名
func extractFileName(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return fmt.Sprintf("file_%d", time.Now().Unix())
}
