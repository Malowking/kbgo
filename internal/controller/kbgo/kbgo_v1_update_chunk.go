package kbgo

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	"github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) UpdateChunk(ctx context.Context, req *v1.UpdateChunkReq) (res *v1.UpdateChunkRes, err error) {
	// Log request parameters
	g.Log().Infof(ctx, "UpdateChunk request received - Ids: %v, Status: %d", req.Ids, req.Status)

	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = errors.Newf(errors.ErrInternalError, "panic occurred during UpdateChunk: %v", r)
		}
	}()

	// 使用事务更新数据库
	err = knowledge.UpdateChunkByIdsWithTx(ctx, tx, req.Ids, gorm.KnowledgeChunks{
		Status: int8(req.Status),
	})
	if err != nil {
		tx.Rollback()
		return nil, errors.Newf(errors.ErrDatabaseUpdate, "failed to update chunk: %v", err)
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, errors.Newf(errors.ErrDatabaseUpdate, "failed to commit transaction: %v", err)
	}

	return &v1.UpdateChunkRes{}, nil
}
