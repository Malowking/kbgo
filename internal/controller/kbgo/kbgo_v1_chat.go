package kbgo

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/chat"
)

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	// 如果启用流式返回，执行流式逻辑
	if req.Stream {
		return nil, c.handleStreamChat(ctx, req)
	}

	// 使用新的聊天处理器
	chatHandler := chat.NewChatHandler()
	return chatHandler.Chat(ctx, req)
}

// handleStreamChat 处理流式聊天请求
func (c *ControllerV1) handleStreamChat(ctx context.Context, req *v1.ChatReq) error {
	// 使用新的流式聊天处理器
	streamHandler := chat.NewStreamHandler()
	return streamHandler.StreamChat(ctx, req)
}
