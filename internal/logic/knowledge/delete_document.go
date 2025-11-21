package knowledge

import (
	"context"
	"fmt"

	"github.com/Malowking/kbgo/internal/dao"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

// DeleteDocumentDataOnly 删除指定文档的所有相关数据，但不删除存储中的文件
// 这包括:
// 1. 删除 knowledge_chunks 表中与该文档相关的所有 chunks
// 2. 删除 Milvus 中与该文档相关的所有向量数据
// 3. 删除 knowledge_documents 表中的文档记录
func DeleteDocumentDataOnly(ctx context.Context, documentId string, milvusClient *milvusclient.Client) error {
	// 开始事务
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取文档信息
	document, err := GetDocumentById(ctx, documentId)
	if err != nil {
		g.Log().Errorf(ctx, "DeleteDocumentDataOnly: GetDocumentById failed for id %s, err: %v", documentId, err)
		tx.Rollback()
		return fmt.Errorf("获取文档信息失败: %w", err)
	}

	// 检查 CollectionName 是否存在
	if document.CollectionName == "" {
		g.Log().Warningf(ctx, "DeleteDocumentDataOnly: CollectionName is empty for document id %s", documentId)
	} else {
		// 使用 DeleteDocument 函数删除 Milvus 中所有该文档的分片
		err = deleteMilvusDocument(ctx, milvusClient, document.CollectionName, documentId)
		if err != nil {
			g.Log().Errorf(ctx, "DeleteDocumentDataOnly: Milvus deleteDocument failed for documentId %s in collection %s, err: %v", documentId, document.CollectionName, err)
			tx.Rollback()
			return fmt.Errorf("删除 Milvus 中的文档数据失败: %w", err)
		}
		g.Log().Infof(ctx, "DeleteDocumentDataOnly: Successfully deleted document %s from Milvus collection %s", documentId, document.CollectionName)
	}

	// 从数据库删除文档记录（会级联删除相关的 chunks）使用事务版本
	err = DeleteDocumentWithTx(ctx, tx, documentId)
	if err != nil {
		g.Log().Errorf(ctx, "DeleteDocumentDataOnly: DeleteDocumentWithTx failed for id %s, err: %v", documentId, err)
		tx.Rollback()
		return fmt.Errorf("删除数据库中的文档数据失败: %w", err)
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "DeleteDocumentDataOnly: transaction commit failed, err: %v", err)
		return fmt.Errorf("提交事务失败: %w", err)
	}

	g.Log().Infof(ctx, "DeleteDocumentDataOnly: Successfully deleted all data for document id %s", documentId)
	return nil
}

// deleteMilvusDocument 从 Milvus 中删除文档的所有 chunks
func deleteMilvusDocument(ctx context.Context, milvusClient *milvusclient.Client, collectionName string, documentID string) error {
	// Build filter expression to match document_id
	filterExpr := fmt.Sprintf(`document_id == "%s"`, documentID)

	g.Log().Infof(ctx, "Attempting to delete document %s from collection %s with filter: %s", documentID, collectionName, filterExpr)

	// Create delete option with filter expression
	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filterExpr)

	// Execute delete operation
	result, err := milvusClient.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("failed to delete document (id=%s) from collection %s: %w", documentID, collectionName, err)
	}

	g.Log().Infof(ctx, "Delete operation completed for document id=%s, affected rows: %d", documentID, result.DeleteCount)

	return nil
}
