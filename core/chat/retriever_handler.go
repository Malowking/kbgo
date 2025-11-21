package chat

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/service"
)

// RetrieverHandler 检索处理器 - 使用共享的服务层
type RetrieverHandler struct{}

// NewRetrieverHandler 创建检索处理器
func NewRetrieverHandler() *RetrieverHandler {
	return &RetrieverHandler{}
}

// ProcessRetrieval 处理知识库检索 - 委托给共享的服务层
func (h *RetrieverHandler) ProcessRetrieval(ctx context.Context, req *v1.RetrieverReq) (*v1.RetrieverRes, error) {
	// 使用共享的检索服务，避免代码重复
	retrieverService := service.GetRetrieverService()
	return retrieverService.ProcessRetrieval(ctx, req)
}
