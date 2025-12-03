package common

import (
	"fmt"
	"strings"
	"time"

	"github.com/Malowking/kbgo/pkg/schema"
)

const systemTemplate = `你非常擅长于使用rag进行数据检索，你的目标是在充分理解用户的问题后进行向量化检索
现在时间%s
你要优化并提取搜索的查询内容。请遵循以下规则重写查询内容：
- 根据用户的问题和上下文，重写应该进行搜索的关键词
- 如果需要使用时间，则根据当前时间给出需要查询的具体时间日期信息
- 保持查询简洁，查询内容通常不超过3个关键词, 最多不要超过5个关键词
- 直接返回优化后的搜索词，不要有任何额外说明。
- 尽量不要使用下面这些已使用过的关键词，因为之前使用这些关键词搜索到的结果不符合预期，已使用过的关键词：%s
- 尽量不使用知识库名字《%s》中包含的关键词`

// GetOptimizedQueryMessages 生成优化查询的消息
func GetOptimizedQueryMessages(used, question, knowledgeBase string) ([]*schema.Message, error) {
	// 构建系统消息
	systemContent := fmt.Sprintf(systemTemplate,
		time.Now().Format(time.RFC3339),
		used,
		knowledgeBase,
	)

	// 构建用户消息
	userContent := fmt.Sprintf("如下是用户的问题: %s", question)

	messages := []*schema.Message{
		{
			Role:    schema.System,
			Content: strings.TrimSpace(systemContent),
		},
		{
			Role:    schema.User,
			Content: userContent,
		},
	}

	return messages, nil
}
