package knowledge

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/model/entity"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// SaveChunksData 批量保存知识块数据
func SaveChunksData(ctx context.Context, documentsId string, chunks []entity.KnowledgeChunks) error {
	if len(chunks) == 0 {
		return nil
	}

	// 使用GORM方式保存，以支持自动填充create_time和update_time字段
	gormChunks := make([]gormModel.KnowledgeChunks, len(chunks))
	for i, chunk := range chunks {
		gormChunks[i] = gormModel.KnowledgeChunks{
			ID:             chunk.Id,
			KnowledgeDocID: chunk.KnowledgeDocId,
			Content:        chunk.Content,
			CollectionName: chunk.CollectionName,
			Ext:            chunk.Ext,
			Status:         int8(chunk.Status),
		}
	}

	// 获取GORM数据库实例
	db := dao.GetDB()

	result := db.WithContext(ctx).CreateInBatches(&gormChunks, len(gormChunks))

	status := int(v1.StatusIndexing)
	if result.Error != nil {
		g.Log().Errorf(ctx, "SaveChunksData err=%+v", result.Error)
		status = int(v1.StatusFailed)
	}

	err := UpdateDocumentsStatus(ctx, documentsId, status)
	if err != nil {
		g.Log().Errorf(ctx, "更新文档状态失败: ID=%s, 错误: %v", documentsId, err)
	}

	return result.Error
}

// GetChunksList 查询知识块列表
func GetChunksList(ctx context.Context, where entity.KnowledgeChunks, page, size int) (list []entity.KnowledgeChunks, total int, err error) {
	model := dao.KnowledgeChunks.Ctx(ctx)

	// 构建查询条件
	if where.KnowledgeDocId != "" {
		model = model.Where("knowledge_doc_id", where.KnowledgeDocId)
	}
	if where.Id != "" {
		model = model.Where("chunk_id", where.Id)
	}

	// 获取总数
	total, err = model.Count()
	if err != nil {
		return
	}

	// 分页查询
	if page > 0 && size > 0 {
		model = model.Page(page, size)
	}

	// 按创建时间倒序
	model = model.OrderDesc("create_time")

	err = model.Scan(&list)
	return
}

// GetChunkById 根据ID查询单个知识块
func GetChunkById(ctx context.Context, id string) (chunk entity.KnowledgeChunks, err error) {
	err = dao.KnowledgeChunks.Ctx(ctx).Where("id", id).Scan(&chunk)
	return
}

// DeleteChunkByIdWithTx 根据ID软删除知识块（事务版本）
func DeleteChunkByIdWithTx(ctx context.Context, tx *gorm.DB, id string) error {
	result := tx.WithContext(ctx).Where("id = ?", id).Delete(&entity.KnowledgeChunks{})
	return result.Error
}

// DeleteChunksByDocumentId 根据文档ID删除该文档的所有chunks（事务版本）
func DeleteChunksByDocumentId(ctx context.Context, tx *gorm.DB, documentId string) error {
	result := tx.WithContext(ctx).Where("knowledge_doc_id = ?", documentId).Delete(&entity.KnowledgeChunks{})
	if result.Error != nil {
		g.Log().Errorf(ctx, "DeleteChunksByDocumentId failed for document %s, err: %v", documentId, result.Error)
		return result.Error
	}
	g.Log().Infof(ctx, "DeleteChunksByDocumentId: Deleted %d chunks for document %s", result.RowsAffected, documentId)
	return nil
}

// UpdateChunkByIdsWithTx 根据ID更新知识块（事务版本）
func UpdateChunkByIdsWithTx(ctx context.Context, tx *gorm.DB, ids []string, data entity.KnowledgeChunks) error {
	updates := make(map[string]interface{})
	if data.Content != "" {
		updates["content"] = data.Content
	}
	if data.Status == 0 || data.Status == 1 {
		updates["status"] = data.Status
	}
	result := tx.WithContext(ctx).Model(&entity.KnowledgeChunks{}).Where("id IN ?", ids).Updates(updates)
	return result.Error
}

// GetAllChunksByDocId gets all chunks by document id
func GetAllChunksByDocId(ctx context.Context, docId string, fields ...string) (list []entity.KnowledgeChunks, err error) {
	model := dao.KnowledgeChunks.Ctx(ctx).Where("knowledge_doc_id", docId)
	if len(fields) > 0 {
		for _, field := range fields {
			model = model.Fields(field)
		}
	}
	err = model.Scan(&list)
	return
}
