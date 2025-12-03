package formatter

import (
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/sashabaranov/go-openai"
)

// MessageFormatter 消息格式适配器接口
// 不同的模型提供商可能有不同的消息格式要求
// 该接口负责将统一的schema.Message转换为OpenAI格式的消息
type MessageFormatter interface {
	// FormatMessages 将schema.Message数组转换为OpenAI格式的消息数组
	FormatMessages(messages []*schema.Message) ([]openai.ChatCompletionMessage, error)
}
