package formatter

import (
	"github.com/Malowking/kbgo/core/schema"
	"github.com/sashabaranov/go-openai"
)

// MessageFormatter 消息格式适配器接口
type MessageFormatter interface {
	// FormatMessages 将schema.Message数组转换为OpenAI格式的消息数组
	FormatMessages(messages []*schema.Message) ([]openai.ChatCompletionMessage, error)
}
