package dao

import (
	"context"

	"github.com/Malowking/kbgo/api/kbgo/v1"
	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// GetActiveChunkIDs 获取活跃状态的chunk ID列表
func GetActiveChunkIDs(ctx context.Context, chunkIDs []string) (map[string]bool, error) {
	if len(chunkIDs) == 0 {
		return make(map[string]bool), nil
	}

	var activeChunks []gormModel.KnowledgeChunks
	err := GetDB().WithContext(ctx).
		Select("id").
		Where("id IN ? AND status = ?", chunkIDs, int8(v1.ChunkStatusActive)).
		Find(&activeChunks).Error

	if err != nil {
		g.Log().Errorf(ctx, "Failed to get active chunk IDs: %v", err)
		return nil, err
	}

	activeIDSet := make(map[string]bool, len(activeChunks))
	for _, chunk := range activeChunks {
		activeIDSet[chunk.ID] = true
	}

	return activeIDSet, nil
}
