package knowledge

import (
	"testing"

	"github.com/Malowking/kbgo/internal/model/entity"
)

// TestExtractChunkOrder 测试从 ext 字段提取 chunk_order
func TestExtractChunkOrder(t *testing.T) {
	tests := []struct {
		name     string
		ext      string
		expected int
	}{
		{
			name:     "Valid JSON with chunk_order",
			ext:      `{"chunk_order": 5}`,
			expected: 5,
		},
		{
			name:     "Valid JSON with chunk_order as zero",
			ext:      `{"chunk_order": 0}`,
			expected: 0,
		},
		{
			name:     "Valid JSON without chunk_order",
			ext:      `{"other_field": "value"}`,
			expected: 999999,
		},
		{
			name:     "Empty string",
			ext:      "",
			expected: 999999,
		},
		{
			name:     "Invalid JSON",
			ext:      `{invalid json}`,
			expected: 999999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractChunkOrder(tt.ext)
			if result != tt.expected {
				t.Errorf("extractChunkOrder(%q) = %d, expected %d", tt.ext, result, tt.expected)
			}
		})
	}
}

// TestSortChunksByOrder 测试按 chunk_order 排序
func TestSortChunksByOrder(t *testing.T) {
	chunks := []entity.KnowledgeChunks{
		{Id: "3", Ext: `{"chunk_order": 2}`},
		{Id: "1", Ext: `{"chunk_order": 0}`},
		{Id: "4", Ext: `{"chunk_order": 3}`},
		{Id: "2", Ext: `{"chunk_order": 1}`},
		{Id: "5", Ext: ``}, // 没有 chunk_order，应该排在最后
	}

	sortChunksByOrder(chunks)

	expectedOrder := []string{"1", "2", "3", "4", "5"}
	for i, chunk := range chunks {
		if chunk.Id != expectedOrder[i] {
			t.Errorf("After sorting, chunks[%d].Id = %s, expected %s", i, chunk.Id, expectedOrder[i])
		}
	}
}
