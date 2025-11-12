package rag

import (
	"context"
	"strings"

	v1 "github.com/Malowking/kbgo/api/rag/v1"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

func (c *ControllerV1) ChunkDelete(ctx context.Context, req *v1.ChunkDeleteReq) (res *v1.ChunkDeleteRes, err error) {
	svr := rag.GetRagSvr()

	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during ChunkDelete: %v", r)
		}
	}()

	chunk, err := knowledge.GetChunkById(ctx, req.ChunkId)
	if err != nil {
		// 如果记录不存在，只输出日志，不返回错误
		if strings.Contains(err.Error(), "no rows in result set") {
			g.Log().Warningf(ctx, "ChunkDelete: chunk id %v not found, it may have been deleted", req.ChunkId)
			tx.Rollback() // 回滚事务
			return &v1.ChunkDeleteRes{}, nil
		}
		// 其他数据库错误
		g.Log().Errorf(ctx, "ChunkDelete: GetChunkById failed for id %v, err: %v", req.ChunkId, err)
		tx.Rollback()
		err = gerror.Newf("failed to query chunk with id %v: %v", req.ChunkId, err)
		return
	}

	// 检查 CollectionName 是否存在
	if chunk.CollectionName == "" {
		g.Log().Warningf(ctx, "ChunkDelete: CollectionName is empty for chunk id %v, skipping Milvus deletion", req.ChunkId)
	} else {
		// 从 Milvus 删除 chunk（根据 chunk 的唯一 ID）
		// 注意：chunk.Id 就是 Milvus 中的主键 id，直接删除即可
		err = svr.DeleteChunk(ctx, chunk.CollectionName, chunk.Id)
		if err != nil {
			g.Log().Errorf(ctx, "ChunkDelete: Milvus DeleteDocument failed for chunk id %v in collection %s, err: %v", chunk.Id, chunk.CollectionName, err)
			tx.Rollback()
			err = gerror.Newf("failed to delete chunk from Milvus: %v", err)
			return
		}
		g.Log().Infof(ctx, "ChunkDelete: Successfully deleted chunk %v from Milvus collection %s", chunk.Id, chunk.CollectionName)
	}

	// 从数据库删除 chunk 记录（使用事务）
	err = knowledge.DeleteChunkByIdWithTx(ctx, tx, req.ChunkId)
	if err != nil {
		g.Log().Errorf(ctx, "ChunkDelete: DeleteChunkById failed for id %v, err: %v", req.ChunkId, err)
		tx.Rollback()
		err = gerror.Newf("failed to delete chunk from database: %v", err)
		return
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "ChunkDelete: transaction commit failed, err: %v", err)
		err = gerror.Newf("failed to commit transaction: %v", err)
		return
	}

	return &v1.ChunkDeleteRes{}, nil
}
