package kbgo

import (
	"context"
	"os"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/core/indexer/file_store"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/logic/index"
	"github.com/Malowking/kbgo/internal/logic/knowledge"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) DocumentsDelete(ctx context.Context, req *v1.DocumentsDeleteReq) (res *v1.DocumentsDeleteRes, err error) {
	docIndexSvr := index.GetDocIndexSvr()

	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			err = gerror.Newf("panic occurred during DocumentsDelete: %v", r)
		}
	}()

	// 从数据库获取文档信息，包括 collection_name
	document, err := knowledge.GetDocumentById(ctx, req.DocumentId)
	if err != nil {
		g.Log().Errorf(ctx, "DocumentsDelete: GetDocumentById failed for id %s, err: %v", req.DocumentId, err)
		tx.Rollback()
		return nil, err
	}

	var needDeleteFromRustFS bool
	var needDeleteLocalFile bool
	var rustfsBucket, rustfsLocation string
	var localFilePath string

	// 检查是否有其他知识库引用了相同的 SHA256 文件
	if document.SHA256 != "" {
		// 查询是否还有其他文档引用相同的 SHA256（使用事务）
		var count int64
		err := tx.WithContext(ctx).Model(&gormModel.KnowledgeDocuments{}).Where("sha256 = ?", document.SHA256).Count(&count).Error
		if err != nil {
			g.Log().Errorf(ctx, "DocumentsDelete: failed to count documents with same SHA256, err: %v", err)
			tx.Rollback()
			return nil, err
		}

		// 如果只有当前这一个文档，则需要删除存储中的文件
		if count == 1 {
			// 根据存储类型决定删除方式
			storageType := file_store.GetStorageType()
			if storageType == file_store.StorageTypeRustFS {
				// 使用 RustFS 存储
				if document.RustfsBucket != "" && document.RustfsLocation != "" {
					needDeleteFromRustFS = true
					rustfsBucket = document.RustfsBucket
					rustfsLocation = document.RustfsLocation
				}
			} else {
				// 使用本地存储
				if document.LocalFilePath != "" {
					needDeleteLocalFile = true
					localFilePath = document.LocalFilePath
				}
			}
		} else {
			g.Log().Infof(ctx, "DocumentsDelete: file is referenced by %d documents, skipping file deletion", count)
		}
	}

	// 检查 CollectionName 是否存在
	if document.CollectionName == "" {
		g.Log().Warningf(ctx, "DocumentsDelete: CollectionName is empty for document id %s, skipping Milvus deletion", req.DocumentId)
	} else {
		// 使用 DeleteDocument 函数删除 Milvus 中所有该文档的分片
		err = docIndexSvr.DeleteDocument(ctx, document.CollectionName, req.DocumentId)
		if err != nil {
			g.Log().Errorf(ctx, "DocumentsDelete: Milvus DeleteDocument failed for documentId %s in collection %s, err: %v", req.DocumentId, document.CollectionName, err)
			tx.Rollback()
			return nil, err
		}
	}

	// 从数据库删除文档记录（会级联删除相关的 chunks）使用事务版本
	err = knowledge.DeleteDocumentWithTx(ctx, tx, req.DocumentId)
	if err != nil {
		g.Log().Errorf(ctx, "DocumentsDelete: DeleteDocument failed for id %s, err: %v", req.DocumentId, err)
		tx.Rollback()
		return nil, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "DocumentsDelete: transaction commit failed, err: %v", err)
		return nil, gerror.Newf("failed to commit transaction: %v", err)
	}

	// 事务成功提交后，删除存储中的文件（这个操作失败不影响数据一致性）
	if needDeleteFromRustFS && rustfsBucket != "" && rustfsLocation != "" {
		g.Log().Infof(ctx, "DocumentsDelete: deleting file from RustFS, bucket=%s, location=%s", rustfsBucket, rustfsLocation)

		rustfsConfig := file_store.GetRustfsConfig()
		err = file_store.DeleteObject(ctx, rustfsConfig.Client, rustfsBucket, rustfsLocation)
		if err != nil {
			g.Log().Errorf(ctx, "DocumentsDelete: failed to delete from RustFS, bucket=%s, location=%s, err: %v", rustfsBucket, rustfsLocation, err)
			// 不返回错误，因为数据库操作已经成功
		} else {
			g.Log().Infof(ctx, "DocumentsDelete: successfully deleted from RustFS, bucket=%s, location=%s", rustfsBucket, rustfsLocation)
		}
	} else if needDeleteLocalFile && localFilePath != "" {
		g.Log().Infof(ctx, "DocumentsDelete: deleting local file, path=%s", localFilePath)

		err = os.Remove(localFilePath)
		if err != nil {
			g.Log().Errorf(ctx, "DocumentsDelete: failed to delete local file, path=%s, err: %v", localFilePath, err)
			// 不返回错误，因为数据库操作已经成功
		} else {
			g.Log().Infof(ctx, "DocumentsDelete: successfully deleted local file, path=%s", localFilePath)
		}
	}

	return &v1.DocumentsDeleteRes{}, nil
}
