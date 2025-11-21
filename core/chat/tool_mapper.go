package chat

import (
	"strings"

	"github.com/Malowking/kbgo/internal/mcp/client"
)

// ToolMapper 工具参数映射器
type ToolMapper struct{}

// NewToolMapper 创建工具映射器
func NewToolMapper() *ToolMapper {
	return &ToolMapper{}
}

// BuildToolArguments 根据工具schema智能构建参数
func (tm *ToolMapper) BuildToolArguments(tool client.MCPTool, question string) (map[string]interface{}, error) {
	args := make(map[string]interface{})

	// 解析工具的输入schema
	if tool.InputSchema == nil {
		return tm.FallbackToolArguments(tool, question), nil
	}

	// 获取properties
	properties, ok := tool.InputSchema["properties"]
	if !ok {
		return tm.FallbackToolArguments(tool, question), nil
	}

	propertiesMap, ok := properties.(map[string]interface{})
	if !ok {
		return tm.FallbackToolArguments(tool, question), nil
	}

	// 遍历每个参数并尝试映射
	for paramName, paramDef := range propertiesMap {
		value := tm.MapParameterValue(paramName, paramDef, question)
		if value != nil {
			args[paramName] = value
		}
	}

	// 如果没有成功映射任何参数，使用fallback策略
	if len(args) == 0 {
		return tm.FallbackToolArguments(tool, question), nil
	}

	return args, nil
}

// MapParameterValue 根据参数名和类型映射值
func (tm *ToolMapper) MapParameterValue(paramName string, paramDef interface{}, question string) interface{} {
	// 将参数名转换为小写进行匹配
	lowerParamName := strings.ToLower(paramName)

	// 根据参数名智能映射
	switch {
	case strings.Contains(lowerParamName, "name"):
		// 尝试从问题中提取人名
		if extractedName := tm.ExtractNameFromQuestion(question); extractedName != "" {
			return extractedName
		}
		return question // fallback
	case strings.Contains(lowerParamName, "question") || strings.Contains(lowerParamName, "query"):
		return question
	case strings.Contains(lowerParamName, "text") || strings.Contains(lowerParamName, "content"):
		return question
	case strings.Contains(lowerParamName, "message") || strings.Contains(lowerParamName, "msg"):
		return question
	default:
		// 对于其他类型的参数，尝试基于类型设置默认值
		paramDefMap, ok := paramDef.(map[string]interface{})
		if !ok {
			return question
		}

		paramType, exists := paramDefMap["type"]
		if !exists {
			return question
		}

		switch paramType {
		case "string":
			return question
		case "boolean":
			return true // 默认true
		case "number", "integer":
			return 1 // 默认1
		default:
			return question
		}
	}
}

// ExtractNameFromQuestion 尝试从问题中提取姓名
func (tm *ToolMapper) ExtractNameFromQuestion(question string) string {
	// 简单的姓名提取逻辑
	question = strings.TrimSpace(question)

	// 如果问题很短且看起来像名字，直接返回
	if len(question) <= 20 && !strings.Contains(question, " ") {
		// 排除一些明显不是名字的词
		lowQuestion := strings.ToLower(question)
		if !strings.Contains(lowQuestion, "how") &&
			!strings.Contains(lowQuestion, "what") &&
			!strings.Contains(lowQuestion, "why") &&
			!strings.Contains(lowQuestion, "when") &&
			!strings.Contains(lowQuestion, "where") {
			return question
		}
	}

	// 查找常见的姓名模式
	words := strings.Fields(question)
	for _, word := range words {
		// 如果单词是大写开头且长度适中，可能是名字
		if len(word) >= 2 && len(word) <= 15 && strings.Title(word) == word {
			// 排除一些常见的非名字单词
			lowWord := strings.ToLower(word)
			if lowWord != "hello" && lowWord != "hi" && lowWord != "the" && lowWord != "my" {
				return word
			}
		}
	}

	return ""
}

// FallbackToolArguments 提供fallback参数映射策略
func (tm *ToolMapper) FallbackToolArguments(tool client.MCPTool, question string) map[string]interface{} {
	// 常见的参数名映射策略
	commonMappings := []string{"question", "query", "text", "content", "message", "input", "name"}

	args := make(map[string]interface{})

	// 尝试每种常见的参数名
	for _, paramName := range commonMappings {
		if paramName == "name" {
			// 特殊处理name参数
			if extractedName := tm.ExtractNameFromQuestion(question); extractedName != "" {
				args[paramName] = extractedName
			} else {
				args[paramName] = question // fallback to question
			}
		} else {
			args[paramName] = question
		}
	}

	return args
}
