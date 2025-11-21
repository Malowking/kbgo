package kbgo

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/gogf/gf/v2/errors/gerror"
)

func (c *ControllerV1) UpdateChunk(ctx context.Context, req *v1.UpdateChunkReq) (res *v1.UpdateChunkRes, err error) {
	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during UpdateChunk: %v", r)
		}
	}()

	// 使用事务更新数据库
	err = knowledge.UpdateChunkByIdsWithTx(ctx, tx, req.Ids, entity.KnowledgeChunks{
		Status: req.Status,
	})
	if err != nil {
		tx.Rollback()
		return
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	return &v1.UpdateChunkRes{}, nil
}
