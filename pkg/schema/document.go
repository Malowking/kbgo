package schema

// Document 表示文档片段
type Document struct {
	// ID 文档唯一标识
	ID string `json:"id,omitempty"`
	// Content 文档内容
	Content string `json:"content"`
	// MetaData 文档元数据
	MetaData map[string]interface{} `json:"metadata,omitempty"`
	// Score 相关性得分（检索时使用）- 使用float32以直接与向量库兼容
	Score float32 `json:"score"`
}
