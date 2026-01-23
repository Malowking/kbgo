package common

import (
	"sort"
	"testing"
)

func TestBM25BasicScoring(t *testing.T) {
	// Create test documents
	docs := []BM25Document{
		{ID: "doc1", Content: "The quick brown fox jumps over the lazy dog"},
		{ID: "doc2", Content: "The lazy dog sleeps all day"},
		{ID: "doc3", Content: "A quick brown fox is very fast"},
		{ID: "doc4", Content: "The fox and the dog are friends"},
	}

	// Create BM25 scorer
	scorer := NewBM25Scorer(docs, DefaultBM25Parameters())

	// Test query
	query := "quick fox"
	results := scorer.Score(query)

	// Documents 1 and 3 should have highest scores (contain both "quick" and "fox")
	if len(results) != 4 {
		t.Errorf("Expected 4 results, got %d", len(results))
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Check that doc1 or doc3 is ranked highest
	if results[0].ID != "doc1" && results[0].ID != "doc3" {
		t.Errorf("Expected doc1 or doc3 to be ranked highest, got %s", results[0].ID)
	}

	// Check that scores are positive for matching docs
	if results[0].Score <= 0 {
		t.Errorf("Expected positive score for matching document")
	}
}

func TestBM25NoMatch(t *testing.T) {
	docs := []BM25Document{
		{ID: "doc1", Content: "The quick brown fox"},
		{ID: "doc2", Content: "The lazy dog sleeps"},
	}

	scorer := NewBM25Scorer(docs, DefaultBM25Parameters())

	// Query with no matching terms
	query := "elephant zebra"
	results := scorer.Score(query)

	// All scores should be 0
	for _, result := range results {
		if result.Score != 0 {
			t.Errorf("Expected score 0 for non-matching query, got %f", result.Score)
		}
	}
}

func TestBM25Normalization(t *testing.T) {
	docs := []BM25Document{
		{ID: "doc1", Score: 10.0},
		{ID: "doc2", Score: 5.0},
		{ID: "doc3", Score: 2.5},
	}

	normalized := NormalizeBM25Scores(docs)

	// Check that max score is 1.0
	maxScore := 0.0
	for _, doc := range normalized {
		if doc.Score > maxScore {
			maxScore = doc.Score
		}
	}

	if maxScore != 1.0 {
		t.Errorf("Expected max normalized score to be 1.0, got %f", maxScore)
	}

	// Check that relative ordering is preserved
	if normalized[0].Score < normalized[1].Score {
		t.Errorf("Normalization changed score ordering")
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "Hello World",
			expected: []string{"hello", "world"},
		},
		{
			input:    "The quick, brown fox!",
			expected: []string{"the", "quick", "brown", "fox"},
		},
		{
			input:    "test123 abc",
			expected: []string{"test123", "abc"},
		},
		{
			input:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		result := tokenize(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("For input '%s', expected %d tokens, got %d", tt.input, len(tt.expected), len(result))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("For input '%s', expected token '%s' at position %d, got '%s'", tt.input, tt.expected[i], i, result[i])
			}
		}
	}
}

func TestBM25EmptyDocuments(t *testing.T) {
	docs := []BM25Document{}
	scorer := NewBM25Scorer(docs, DefaultBM25Parameters())

	results := scorer.Score("test query")

	if len(results) != 0 {
		t.Errorf("Expected 0 results for empty document set, got %d", len(results))
	}
}

func TestBM25ChineseText(t *testing.T) {
	// Test with Chinese text
	docs := []BM25Document{
		{ID: "doc1", Content: "这是一个测试文档"},
		{ID: "doc2", Content: "另一个测试文档"},
		{ID: "doc3", Content: "完全不同的内容"},
	}

	scorer := NewBM25Scorer(docs, DefaultBM25Parameters())

	// Note: Current tokenizer splits by spaces, so Chinese characters
	// will be treated as single tokens. This is a simplified implementation.
	// For production, you may want to use a proper Chinese tokenizer.
	query := "测试"
	results := scorer.Score(query)

	if len(results) != 3 {
		t.Errorf("Expected 3 results, got %d", len(results))
	}
}
