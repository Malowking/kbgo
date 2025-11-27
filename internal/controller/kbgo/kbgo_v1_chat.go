package kbgo

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/chat"
	"github.com/Malowking/kbgo/core/common"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "Chat request received - ConvID: %s, Question: %s, KnowledgeId: %s, EnableRetriever: %v, TopK: %d, Score: %f, UseMCP: %v, Stream: %v",
		req.ConvID, req.Question, req.KnowledgeId, req.EnableRetriever, req.TopK, req.Score, req.UseMCP, req.Stream)

	// 异步处理文件上传
	var uploadedFiles []*common.MultimodalFile
	if len(req.Files) > 0 {
		g.Log().Infof(ctx, "Processing %d uploaded files asynchronously", len(req.Files))

		// 使用全局文件上传器
		fileUploader := common.GetGlobalFileUploader()

		uploadedFiles, err = fileUploader.UploadFiles(ctx, req.Files)
		if err != nil {
			g.Log().Errorf(ctx, "Error during file upload: %v", err)
		}

		g.Log().Infof(ctx, "Successfully uploaded %d files", len(uploadedFiles))
	}

	// 如果启用流式返回，执行流式逻辑
	if req.Stream {
		return nil, c.handleStreamChat(ctx, req, uploadedFiles)
	}

	// 使用新的聊天处理器
	chatHandler := chat.NewChatHandler()
	return chatHandler.Chat(ctx, req, uploadedFiles)
}

// handleStreamChat 处理流式聊天请求
func (c *ControllerV1) handleStreamChat(ctx context.Context, req *v1.ChatReq, uploadedFiles []*common.MultimodalFile) error {
	// Log request parameters
	g.Log().Infof(ctx, "Stream chat request received - ConvID: %s, Question: %s, KnowledgeId: %s, EnableRetriever: %v, TopK: %d, Score: %f, UseMCP: %v",
		req.ConvID, req.Question, req.KnowledgeId, req.EnableRetriever, req.TopK, req.Score, req.UseMCP)

	// 使用新的流式聊天处理器
	streamHandler := chat.NewStreamHandler()
	return streamHandler.StreamChat(ctx, req, uploadedFiles)
}
