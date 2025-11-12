package rag

import (
	"context"
	"time"

	v1 "github.com/Malowking/kbgo/api/rag/v1"
	gorag "github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/bytedance/sonic"
	"github.com/cloudwego/eino/schema"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gctx"
)

func (c *ControllerV1) UpdateChunkContent(ctx context.Context, req *v1.UpdateChunkContentReq) (res *v1.UpdateChunkContentRes, err error) {
	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during UpdateChunkContent: %v", r)
		}
	}()

	chunk, err := knowledge.GetChunkById(ctx, req.Id)
	if err != nil {
		g.Log().Errorf(ctx, "GetChunkById failed, err=%v", err)
		tx.Rollback()
		return
	}

	document, err := knowledge.GetDocumentById(ctx, chunk.KnowledgeDocId)
	if err != nil {
		g.Log().Errorf(ctx, "GetDocumentById failed, err=%v", err)
		tx.Rollback()
		return
	}

	knowledgeId := document.KnowledgeId
	collectionName := document.CollectionName

	// 使用事务更新数据库
	err = knowledge.UpdateChunkByIdsWithTx(ctx, tx, []string{req.Id}, entity.KnowledgeChunks{
		Content: req.Content,
	})
	if err != nil {
		g.Log().Errorf(ctx, "UpdateChunkByIds failed, err=%v", err)
		tx.Rollback()
		return
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "UpdateChunkContent: transaction commit failed, err: %v", err)
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	// 异步更新向量索引（不影响数据一致性）
	go func() {
		// 等待一段时间确保数据库更新完成
		time.Sleep(time.Millisecond * 500)

		ctxN := gctx.New()
		defer func() {
			if e := recover(); e != nil {
				g.Log().Errorf(ctxN, "recover updateChunkContent failed, err=%v", e)
			}
		}()

		doc := &schema.Document{
			ID:      chunk.Id,
			Content: req.Content,
		}

		if chunk.Ext != "" {
			extData := map[string]any{}
			if err := sonic.Unmarshal([]byte(chunk.Ext), &extData); err == nil {
				doc.MetaData = extData
			}
		}

		// 调用异步索引更新
		ragSvr := rag.GetRagSvr()
		asyncReq := &gorag.IndexAsyncReq{
			Docs:           []*schema.Document{doc},
			KnowledgeId:    knowledgeId,
			CollectionName: collectionName,
			DocumentsId:    chunk.KnowledgeDocId,
		}

		_, err = ragSvr.IndexAsync(ctxN, asyncReq)
		if err != nil {
			g.Log().Errorf(ctxN, "IndexAsync failed, err=%v", err)
		} else {
			g.Log().Infof(ctxN, "Chunk content updated and reindexed successfully, chunk_id=%s", req.Id)
		}
	}()

	return &v1.UpdateChunkContentRes{}, nil
}
