package kbgo

import (
	"context"
	"fmt"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/gogf/gf/v2/frame/g"
)

// IndexDocuments 文件索引接口（批量切分并向量化）
func (c *ControllerV1) IndexDocuments(ctx context.Context, req *v1.IndexDocumentsReq) (res *v1.IndexDocumentsRes, err error) {
	// 异步处理索引任务
	go func() {
		asyncCtx := context.Background()
		asyncIndexDocuments(asyncCtx, req)
	}()

	res = &v1.IndexDocumentsRes{
		Message: fmt.Sprintf("已启动 %d 个文档的索引任务", len(req.DocumentIds)),
	}
	return
}

// 异步处理文档索引
func asyncIndexDocuments(ctx context.Context, req *v1.IndexDocumentsReq) {
	g.Log().Infof(ctx, "开始批量索引文档，文档数量：%d", len(req.DocumentIds))

	// 获取 Rag 服务实例
	ragSvr := rag.GetRagSvr()

	// 构建批量索引请求参数
	batchReq := &core.BatchIndexReq{
		DocumentIds: req.DocumentIds,
		ChunkSize:   req.ChunkSize,
		OverlapSize: req.OverlapSize,
		Separator:   req.Separator,
	}

	// 使用 BatchDocumentIndex 方法处理批量索引
	err := ragSvr.BatchDocumentIndex(ctx, batchReq)
	if err != nil {
		g.Log().Errorf(ctx, "批量文档索引处理失败, err=%v", err)
		return
	}

	g.Log().Infof(ctx, "批量索引文档任务已启动")
}
