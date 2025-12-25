package kbgo

import (
	"context"
	"fmt"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/indexer"
	"github.com/Malowking/kbgo/internal/logic/index"
	"github.com/gogf/gf/v2/frame/g"
)

// IndexDocuments 文件索引接口
func (c *ControllerV1) IndexDocuments(ctx context.Context, req *v1.IndexDocumentsReq) (res *v1.IndexDocumentsRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "IndexDocuments request received - EmbeddingModelID: %s, DocumentIds: %v, ChunkSize: %d, OverlapSize: %d, Separator: '%s'",
		req.EmbeddingModelID, req.DocumentIds, req.ChunkSize, req.OverlapSize, req.Separator)

	g.Log().Infof(ctx, "收到批量索引请求，文档数量: %d", len(req.DocumentIds))

	// 获取文档索引服务实例
	docIndexSvr := index.GetDocIndexSvr()

	// 构建批量索引请求参数
	batchReq := &indexer.BatchIndexReq{
		ModelID:     req.EmbeddingModelID,
		DocumentIds: req.DocumentIds,
		ChunkSize:   req.ChunkSize,
		OverlapSize: req.OverlapSize,
		Separator:   req.Separator,
	}

	// 异步启动批量索引任务
	go func() {
		asyncCtx := context.Background()
		g.Log().Infof(asyncCtx, "开始异步批量索引文档，文档数量: %d", len(req.DocumentIds))

		// 使用 BatchDocumentIndex 方法处理批量索引
		err := docIndexSvr.BatchDocumentIndex(asyncCtx, batchReq)
		if err != nil {
			g.Log().Errorf(asyncCtx, "批量文档索引启动失败, err=%v", err)
			return
		}

		g.Log().Infof(asyncCtx, "批量索引任务已成功启动")
	}()

	// 立即返回响应
	res = &v1.IndexDocumentsRes{
		Message: fmt.Sprintf("已启动 %d 个文档的索引任务", len(req.DocumentIds)),
	}
	return
}
