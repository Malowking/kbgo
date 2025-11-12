package knowledge

import (
	"context"

	v1 "github.com/Malowking/kbgo/api/rag/v1"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/Malowking/kbgo/internal/model/entity"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// SaveChunksData 批量保存知识块数据
func SaveChunksData(ctx context.Context, documentsId string, chunks []entity.KnowledgeChunks) error {
	if len(chunks) == 0 {
		return nil
	}
	status := int(v1.StatusIndexing)
	_, err := dao.KnowledgeChunks.Ctx(ctx).Data(chunks).Save()
	if err != nil {
		g.Log().Errorf(ctx, "SaveChunksData err=%+v", err)
		status = int(v1.StatusFailed)
	}
	err = UpdateDocumentsStatus(ctx, documentsId, status)
	if err != nil {
		g.Log().Errorf(ctx, "更新文档状态失败: ID=%d, 错误: %v", documentsId, err)
	}
	return err
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
	model = model.OrderDesc("created_at")

	err = model.Scan(&list)
	return
}

// GetChunkById 根据ID查询单个知识块
func GetChunkById(ctx context.Context, id string) (chunk entity.KnowledgeChunks, err error) {
	err = dao.KnowledgeChunks.Ctx(ctx).Where("id", id).Scan(&chunk)
	return
}

// DeleteChunkByIds 根据ID软删除知识块
func DeleteChunkById(ctx context.Context, id string) error {
	_, err := dao.KnowledgeChunks.Ctx(ctx).Where("id", id).Delete()
	return err
}

// DeleteChunkByIdWithTx 根据ID软删除知识块（事务版本）
func DeleteChunkByIdWithTx(ctx context.Context, tx *gorm.DB, id string) error {
	result := tx.WithContext(ctx).Where("id = ?", id).Delete(&entity.KnowledgeChunks{})
	return result.Error
}

// UpdateChunkById 根据ID更新知识块
func UpdateChunkByIds(ctx context.Context, ids []string, data entity.KnowledgeChunks) error {
	model := dao.KnowledgeChunks.Ctx(ctx).WhereIn("id", ids)
	if data.Content != "" {
		model = model.Data("content", data.Content)
	}
	if data.Status != 0 {
		model = model.Data("status", data.Status)
	}
	_, err := model.Update()
	return err
}

// UpdateChunkByIdsWithTx 根据ID更新知识块（事务版本）
func UpdateChunkByIdsWithTx(ctx context.Context, tx *gorm.DB, ids []string, data entity.KnowledgeChunks) error {
	updates := make(map[string]interface{})
	if data.Content != "" {
		updates["content"] = data.Content
	}
	if data.Status != 0 {
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
