package rag

import (
	"context"
	"strings"

	"github.com/Malowking/kbgo/api/rag/v1"
	gorag "github.com/Malowking/kbgo/core"
	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/rag"
	_ "github.com/Malowking/kbgo/internal/logic/rag"
	"github.com/Malowking/kbgo/internal/model/do"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/google/uuid"
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

	// 先创建数据库记录（GORM 会自动设置 CreateTime 和 UpdateTime）
	kb := gormModel.KnowledgeBase{
		ID:             id,
		Name:           req.Name,
		CollectionName: id,
		Status:         int(v1.StatusOK),
		Description:    req.Description,
		Category:       req.Category,
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

	// 1. 获取该知识库下的所有文档信息（用于删除 RustFS 文件）
	var documents []gormModel.KnowledgeDocuments
	result := tx.WithContext(ctx).Where("knowledge_id = ?", req.Id).Find(&documents)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 2. 收集需要删除的 RustFS 文件信息（去重）
	type RustFSFile struct {
		Bucket   string
		Location string
		SHA256   string
	}
	rustfsFiles := make(map[string]RustFSFile) // 使用 map 去重

	for _, doc := range documents {
		if doc.SHA256 != "" && doc.RustfsBucket != "" && doc.RustfsLocation != "" {
			rustfsFiles[doc.SHA256] = RustFSFile{
				Bucket:   doc.RustfsBucket,
				Location: doc.RustfsLocation,
				SHA256:   doc.SHA256,
			}
		}
	}

	// 3. 删除该知识库下所有文档的 chunks
	for _, doc := range documents {
		result = tx.WithContext(ctx).Where("knowledge_doc_id = ?", doc.ID).Delete(&gormModel.KnowledgeChunks{})
		if result.Error != nil {
			tx.Rollback()
			return nil, result.Error
		}
	}

	// 4. 删除该知识库下的所有文档记录
	result = tx.WithContext(ctx).Where("knowledge_id = ?", req.Id).Delete(&gormModel.KnowledgeDocuments{})
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 5. 删除知识库记录
	result = tx.WithContext(ctx).Where("id = ?", req.Id).Delete(&gormModel.KnowledgeBase{})
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 6. 删除 Milvus collection
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

	// 7. 事务成功提交后，删除 RustFS 文件（这个操作失败不影响数据一致性）
	if len(rustfsFiles) > 0 {
		rustfsConfig := gorag.GetRustfsConfig()
		rustfsClient := rustfsConfig.Client

		for _, file := range rustfsFiles {
			err = common.DeleteObject(ctx, rustfsClient, file.Bucket, file.Location)
			if err != nil {
				// 记录错误但不返回，因为数据库操作已经成功
				_ = err // 避免未使用变量的警告
			}
		}
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
	result := tx.WithContext(ctx).Model(&gormModel.KnowledgeBase{}).Where("id = ?", req.Id).Updates(updateData)
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

func (c *ControllerV1) KBUpdateStatus(ctx context.Context, req *v1.KBUpdateStatusReq) (res *v1.KBUpdateStatusRes, err error) {
	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during KBUpdateStatus: %v", r)
		}
	}()

	// 检查知识库是否存在
	var count int64
	result := tx.WithContext(ctx).Model(&gormModel.KnowledgeBase{}).Where("id = ?", req.Id).Count(&count)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}
	if count == 0 {
		tx.Rollback()
		return nil, gerror.Newf("knowledge base not found: %s", req.Id)
	}

	// 更新状态
	result = tx.WithContext(ctx).Model(&gormModel.KnowledgeBase{}).Where("id = ?", req.Id).Update("status", req.Status)
	if result.Error != nil {
		tx.Rollback()
		return nil, result.Error
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	return &v1.KBUpdateStatusRes{}, nil
}
