package rag

import (
	"context"
	"strings"

	"github.com/Malowking/kbgo/api/rag/v1"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/rag"
	_ "github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/Malowking/kbgo/internal/model/do"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (c *ControllerV1) KBCreate(ctx context.Context, req *v1.KBCreateReq) (res *v1.KBCreateRes, err error) {
	id := "kbgo_" + strings.ReplaceAll(uuid.New().String(), "-", "")

	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during KBCreate: %v", r)
		}
	}()

	// 先创建数据库记录
	kb := do.KnowledgeBase{
		Id:             id,
		Name:           req.Name,
		CollectionName: id,
		//QACollectionName: "qa_" + id,
		Status:      v1.StatusOK,
		Description: req.Description,
		Category:    req.Category,
	}
	result := tx.WithContext(ctx).Create(&kb)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 创建 Milvus collection
	ragSvrM := rag.GetRagSvr()
	milvusClient := ragSvrM.Client
	err = common.CreateCollection(ctx, milvusClient, id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	// TODO未来修改添加QA生成功能
	//err = common.CreateQACollection(ctx, milvusClient, "qa_"+id)
	//if err != nil {
	//	return nil, err
	//}

	//go func() {
	//	ragSvrM := rag.GetRagSvrM()
	//	milvusClient := ragSvrM.Client
	//	err = common.CreateTextCollection(context.Background(), milvusClient, req.Name)
	//	if err != nil {
	//		g.Log().Error(ctx, "CreateTextCollection failed, err=%v", err)
	//		return
	//	}
	//
	//	err = common.CreateQACollection(context.Background(), milvusClient, req.Name)
	//	if err != nil {
	//		g.Log().Error(ctx, "CreateQACollection failed, err=%v", err)
	//		return
	//	}
	//}()

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		// 如果数据库事务提交失败，尝试删除已创建的 Milvus collection
		deleteErr := common.DeleteCollection(ctx, milvusClient, id)
		if deleteErr != nil {
			// 记录清理失败的日志，但不覆盖原始错误
		}
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	res = &v1.KBCreateRes{
		Id: id,
	}
	return
}

func (c *ControllerV1) KBDelete(ctx context.Context, req *v1.KBDeleteReq) (res *v1.KBDeleteRes, err error) {
	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during KBDelete: %v", r)
		}
	}()

	// 先从数据库删除记录
	result := tx.WithContext(ctx).Where("id = ?", req.Id).Delete(&do.KnowledgeBase{})
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 删除 Milvus collection
	ragSvrM := rag.GetRagSvr()
	milvusClient := ragSvrM.Client
	err = common.DeleteCollection(ctx, milvusClient, req.Id)
	if err != nil {
		tx.Rollback()
		return nil, err
	}
	//err = common.DeleteCollection(ctx, milvusClient, "qa_"+req.Id)
	//if err != nil {
	//	tx.Rollback()
	//	return nil, err
	//}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	return &v1.KBDeleteRes{}, nil
}

func (c *ControllerV1) KBGetList(ctx context.Context, req *v1.KBGetListReq) (res *v1.KBGetListRes, err error) {
	res = &v1.KBGetListRes{}
	err = dao.KnowledgeBase.Ctx(ctx).Where(do.KnowledgeBase{
		Status:   req.Status,
		Name:     req.Name,
		Category: req.Category,
	}).Scan(&res.List)
	return
}

func (c *ControllerV1) KBGetOne(ctx context.Context, req *v1.KBGetOneReq) (res *v1.KBGetOneRes, err error) {
	res = &v1.KBGetOneRes{}
	err = dao.KnowledgeBase.Ctx(ctx).WherePri(req.Id).Scan(&res.KnowledgeBase)
	return
}

func (c *ControllerV1) KBUpdate(ctx context.Context, req *v1.KBUpdateReq) (res *v1.KBUpdateRes, err error) {
	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during KBUpdate: %v", r)
		}
	}()

	// 更新数据库记录
	updateData := map[string]interface{}{
		"name":        req.Name,
		"status":      req.Status,
		"description": req.Description,
		"category":    req.Category,
	}
	result := tx.WithContext(ctx).Model(&do.KnowledgeBase{}).Where("id = ?", req.Id).Updates(updateData)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	return &v1.KBUpdateRes{}, nil
}
