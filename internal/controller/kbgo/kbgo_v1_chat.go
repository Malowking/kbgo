package kbgo

import (
	"context"
	"mime/multipart"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/chat"
	"github.com/Malowking/kbgo/core/common"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) Chat(ctx context.Context, req *v1.ChatReq) (res *v1.ChatRes, err error) {
	// Log request parameters
	hasTools := req.Tools != nil && len(req.Tools) > 0
	g.Log().Infof(ctx, "Chat request received - ConvID: %s, Question: %s, ModelID: %s, RerankModelID: %s, KnowledgeId: %s, EnableRetriever: %v, TopK: %d, Score: %f, HasTools: %v, Stream: %v",
		req.ConvID, req.Question, req.ModelID, req.RerankModelID, req.KnowledgeId, req.EnableRetriever, req.TopK, req.Score, hasTools, req.Stream)

	r := g.RequestFromCtx(ctx)
	uploadFiles := r.GetUploadFiles("files")

	g.Log().Infof(ctx, "Manual file check - Found %d files from request", len(uploadFiles))

	// 将 ghttp.UploadFiles 转换为 []*multipart.FileHeader
	var fileHeaders []*multipart.FileHeader
	if len(uploadFiles) > 0 {
		for _, uploadFile := range uploadFiles {
			if uploadFile != nil && uploadFile.FileHeader != nil {
				fileHeaders = append(fileHeaders, uploadFile.FileHeader)
			}
		}
	}

	// 异步处理文件上传
	var uploadedFiles []*common.MultimodalFile
	if len(fileHeaders) > 0 {
		g.Log().Infof(ctx, "Processing %d uploaded files asynchronously", len(fileHeaders))

		// 打印每个文件的详细信息
		for i, file := range fileHeaders {
			if file == nil {
				g.Log().Warningf(ctx, "File %d is nil", i)
			} else {
				g.Log().Infof(ctx, "File %d: Filename='%s', Size=%d", i, file.Filename, file.Size)
			}
		}

		// 使用全局文件上传器
		fileUploader := common.GetGlobalFileUploader()

		uploadedFiles, err = fileUploader.UploadFiles(ctx, fileHeaders)
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
	hasTools := req.Tools != nil && len(req.Tools) > 0
	g.Log().Infof(ctx, "Stream chat request received - ConvID: %s, Question: %s, ModelID: %s, RerankModelID: %s, KnowledgeId: %s, EnableRetriever: %v, TopK: %d, Score: %f, HasTools: %v, Files: %d",
		req.ConvID, req.Question, req.ModelID, req.RerankModelID, req.KnowledgeId, req.EnableRetriever, req.TopK, req.Score, hasTools, len(req.Files))

	// 使用新的流式聊天处理器
	streamHandler := chat.NewStreamHandler()
	return streamHandler.StreamChat(ctx, req, uploadedFiles)
}
