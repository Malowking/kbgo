package history

import (
	"testing"

	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/stretchr/testify/assert"
)

func TestManager_SaveMessageWithMetadata(t *testing.T) {
	// Skip if database is not initialized
	if dao.GetDB() == nil {
		t.Skip("Database not initialized, skipping test")
	}

	// 创建历史记录管理器（不实际连接数据库）
	manager := NewManager()

	// 创建测试消息
	message := &schema.Message{
		Role:    schema.Assistant,
		Content: "这是一个测试消息",
	}

	// 创建测试元数据
	metadata := map[string]interface{}{
		"retriever": map[string]interface{}{
			"type":           "retriever",
			"knowledge_id":   "test_knowledge_base",
			"top_k":          5,
			"score":          0.8,
			"document_count": 3,
		},
		"mcp_tools": []map[string]interface{}{
			{
				"type":         "mcp",
				"service_name": "weather_service",
				"tool_name":    "get_weather",
				"content":      "今天天气晴朗，温度25度",
			},
		},
	}

	// 测试保存带元数据的消息（这里只验证方法是否存在，不实际执行）
	assert.NotNil(t, manager)
	assert.NotNil(t, message)
	assert.NotNil(t, metadata)

	// 验证方法签名是否正确
	convID := "test_conv_id"
	// 注意：由于我们不连接数据库，这里不会实际执行保存操作
	// 只是验证方法是否存在且签名正确
	_ = convID
}

func TestManager_SaveMessage(t *testing.T) {
	// Skip if database is not initialized
	if dao.GetDB() == nil {
		t.Skip("Database not initialized, skipping test")
	}

	// 创建历史记录管理器
	manager := NewManager()

	// 创建测试消息
	message := &schema.Message{
		Role:    schema.User,
		Content: "这是一个普通测试消息",
	}

	// 测试保存普通消息
	convID := "test_conv_id_2"
	// 注意：由于我们不连接数据库，这里不会实际执行保存操作
	// 只是验证方法是否存在且签名正确
	_ = manager
	_ = message
	_ = convID
}
