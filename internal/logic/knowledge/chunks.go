package knowledge

import (
	"context"
	"encoding/json"
	"sort"
	"strings"

	v1 "github.com/Malowking/kbgo/api/kbgo/v1"
	"github.com/Malowking/kbgo/internal/dao"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
	"gorm.io/gorm"
)

// SaveChunksData 批量保存知识块数据
func SaveChunksData(ctx context.Context, documentsId string, chunks []gormModel.KnowledgeChunks) error {
	if len(chunks) == 0 {
		return nil
	}

	// 使用GORM方式保存，以支持自动填充create_time和update_time字段
	gormChunks := make([]gormModel.KnowledgeChunks, len(chunks))
	for i, chunk := range chunks {
		// 构建 ext 字段，添加顺序信息
		extData := make(map[string]interface{})

		// 如果原有 ext 不为空，先解析
		if chunk.Ext != "" {
			if err := json.Unmarshal([]byte(chunk.Ext), &extData); err != nil {
				g.Log().Warningf(ctx, "Failed to parse existing ext field for chunk %s: %v", chunk.ID, err)
				extData = make(map[string]interface{})
			}
		}

		// 添加顺序字段（从0开始）
		extData["chunk_order"] = i

		// 序列化为 JSON 字符串
		extJSON, err := json.Marshal(extData)
		if err != nil {
			g.Log().Errorf(ctx, "Failed to marshal ext data for chunk %s: %v", chunk.ID, err)
			extJSON = []byte("{}")
		}

		gormChunks[i] = gormModel.KnowledgeChunks{
			ID:             chunk.ID,
			KnowledgeDocID: chunk.KnowledgeDocID,
			Content:        sanitizeText(chunk.Content), // 清理NULL字节
			CollectionName: chunk.CollectionName,
			Ext:            string(extJSON),
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
func GetChunksList(ctx context.Context, where gormModel.KnowledgeChunks, page, size int) (list []gormModel.KnowledgeChunks, total int, err error) {
	query := dao.GetDB().WithContext(ctx).Model(&gormModel.KnowledgeChunks{})

	// 构建查询条件
	if where.KnowledgeDocID != "" {
		query = query.Where("knowledge_doc_id = ?", where.KnowledgeDocID)
	}
	if where.ID != "" {
		query = query.Where("id = ?", where.ID)
	}

	// 获取总数
	var count int64
	err = query.Count(&count).Error
	total = int(count)
	if err != nil {
		return
	}

	// 查询所有数据（不分页），以便按 ext 中的 chunk_order 排序
	var allList []gormModel.KnowledgeChunks
	err = query.Find(&allList).Error
	if err != nil {
		return
	}

	// 按照 ext 中的 chunk_order 字段排序
	sortChunksByOrder(allList)

	// 手动分页
	if page > 0 && size > 0 {
		start := (page - 1) * size
		end := start + size

		if start >= len(allList) {
			// 起始位置超出范围，返回空列表
			list = []gormModel.KnowledgeChunks{}
		} else {
			if end > len(allList) {
				end = len(allList)
			}
			list = allList[start:end]
		}
	} else {
		list = allList
	}

	return
}

// GetChunkById 根据ID查询单个知识块
func GetChunkById(ctx context.Context, id string) (chunk gormModel.KnowledgeChunks, err error) {
	err = dao.GetDB().WithContext(ctx).Where("id = ?", id).First(&chunk).Error
	return
}

// DeleteChunkByIdWithTx 根据ID删除知识块（事务版本）
func DeleteChunkByIdWithTx(ctx context.Context, tx *gorm.DB, id string) error {
	result := tx.WithContext(ctx).Where("id = ?", id).Delete(&gormModel.KnowledgeChunks{})
	return result.Error
}

// DeleteChunksByDocumentId 根据文档ID删除该文档的所有chunks（事务版本）
func DeleteChunksByDocumentId(ctx context.Context, tx *gorm.DB, documentId string) error {
	result := tx.WithContext(ctx).Where("knowledge_doc_id = ?", documentId).Delete(&gormModel.KnowledgeChunks{})
	if result.Error != nil {
		g.Log().Errorf(ctx, "DeleteChunksByDocumentId failed for document %s, err: %v", documentId, result.Error)
		return result.Error
	}
	g.Log().Infof(ctx, "DeleteChunksByDocumentId: Deleted %d chunks for document %s", result.RowsAffected, documentId)
	return nil
}

// UpdateChunkByIdsWithTx 根据ID更新知识块（事务版本）
func UpdateChunkByIdsWithTx(ctx context.Context, tx *gorm.DB, ids []string, data gormModel.KnowledgeChunks) error {
	updates := make(map[string]interface{})
	if data.Content != "" {
		updates["content"] = data.Content
	}
	if data.Status == 0 || data.Status == 1 {
		updates["status"] = data.Status
	}
	result := tx.WithContext(ctx).Model(&gormModel.KnowledgeChunks{}).Where("id IN ?", ids).Updates(updates)
	return result.Error
}

// GetAllChunksByDocId gets all chunks by document id
func GetAllChunksByDocId(ctx context.Context, docId string, fields ...string) (list []gormModel.KnowledgeChunks, err error) {
	query := dao.GetDB().WithContext(ctx).Model(&gormModel.KnowledgeChunks{}).Where("knowledge_doc_id = ?", docId)

	// GORM doesn't support selecting specific fields when scanning into structs the same way
	// If you need specific fields, use Select()
	if len(fields) > 0 {
		query = query.Select(fields)
	}

	err = query.Find(&list).Error
	if err != nil {
		return
	}

	// 按照 ext 中的 chunk_order 字段排序
	sortChunksByOrder(list)
	return
}

// sortChunksByOrder 根据 ext 字段中的 chunk_order 对 chunks 进行排序
func sortChunksByOrder(chunks []gormModel.KnowledgeChunks) {
	sort.Slice(chunks, func(i, j int) bool {
		orderI := extractChunkOrder(chunks[i].Ext)
		orderJ := extractChunkOrder(chunks[j].Ext)
		return orderI < orderJ
	})
}

// extractChunkOrder 从 ext 字段中提取 chunk_order 的值
// 如果提取失败或不存在，返回一个很大的数字以保证排在最后
func extractChunkOrder(ext string) int {
	if ext == "" {
		return 999999
	}

	var extData map[string]interface{}
	if err := json.Unmarshal([]byte(ext), &extData); err != nil {
		return 999999
	}

	if order, ok := extData["chunk_order"]; ok {
		// JSON 数字会被解析为 float64
		if orderFloat, ok := order.(float64); ok {
			return int(orderFloat)
		}
		// 如果是整数类型
		if orderInt, ok := order.(int); ok {
			return orderInt
		}
	}

	return 999999
}

// sanitizeText 清理文本中的NULL字节和其他PostgreSQL不支持的字符
// PostgreSQL不允许在TEXT/VARCHAR字段中存储NULL字节(0x00)
func sanitizeText(text string) string {
	// 移除NULL字节(0x00)
	return strings.ReplaceAll(text, "\x00", "")
}
