package dao

import (
	"context"

	gormModel "github.com/Malowking/kbgo/internal/model/gorm"
	"github.com/gogf/gf/v2/frame/g"
)

// GetActiveChunkIDs 获取活跃状态的chunk ID列表
// status = 2 表示活跃状态 (StatusActive)
// 返回一个map[string]bool用作set，方便快速查找
func GetActiveChunkIDs(ctx context.Context, chunkIDs []string) (map[string]bool, error) {
	if len(chunkIDs) == 0 {
		return make(map[string]bool), nil
	}

	var activeChunks []gormModel.KnowledgeChunks
	err := GetDB().WithContext(ctx).
		Select("id").
		Where("id IN ? AND status = ?", chunkIDs, int8(2)). // 2 = StatusActive
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
