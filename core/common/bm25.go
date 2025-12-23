package common

import (
	"math"
	"strings"
	"unicode"
)

// BM25Parameters holds the parameters for the BM25 algorithm
type BM25Parameters struct {
	K1 float64 // Term frequency saturation parameter (default: 1.5)
	B  float64 // Length normalization parameter (default: 0.75)
}

// DefaultBM25Parameters returns the default BM25 parameters
func DefaultBM25Parameters() BM25Parameters {
	return BM25Parameters{
		K1: 1.5,
		B:  0.75,
	}
}

// BM25Document represents a document for BM25 scoring
type BM25Document struct {
	ID      string
	Content string
	Score   float64
}

// BM25Scorer implements the BM25 ranking algorithm
type BM25Scorer struct {
	params      BM25Parameters
	avgDocLen   float64
	docFreq     map[string]int   // Number of documents containing each term
	totalDocs   int              // Total number of documents
	docTermFreq []map[string]int // Term frequencies for each document
	docLengths  []int            // Length of each document
	docIDs      []string         // Document IDs
}

// NewBM25Scorer creates a new BM25 scorer with the given documents
func NewBM25Scorer(documents []BM25Document, params BM25Parameters) *BM25Scorer {
	scorer := &BM25Scorer{
		params:      params,
		totalDocs:   len(documents),
		docFreq:     make(map[string]int),
		docTermFreq: make([]map[string]int, len(documents)),
		docLengths:  make([]int, len(documents)),
		docIDs:      make([]string, len(documents)),
	}

	// Preprocess all documents
	totalLen := 0
	for i, doc := range documents {
		scorer.docIDs[i] = doc.ID
		terms := tokenize(doc.Content)
		scorer.docLengths[i] = len(terms)
		totalLen += len(terms)

		// Count term frequencies in this document
		termFreq := make(map[string]int)
		for _, term := range terms {
			termFreq[term]++
		}
		scorer.docTermFreq[i] = termFreq

		// Update document frequency for each unique term
		for term := range termFreq {
			scorer.docFreq[term]++
		}
	}

	// Calculate average document length
	if scorer.totalDocs > 0 {
		scorer.avgDocLen = float64(totalLen) / float64(scorer.totalDocs)
	}

	return scorer
}

// Score calculates BM25 scores for all documents given a query
func (s *BM25Scorer) Score(query string) []BM25Document {
	queryTerms := tokenize(query)
	results := make([]BM25Document, s.totalDocs)

	for i := 0; i < s.totalDocs; i++ {
		score := 0.0
		docLen := float64(s.docLengths[i])

		for _, term := range queryTerms {
			// Get term frequency in document
			tf, exists := s.docTermFreq[i][term]
			if !exists {
				continue
			}

			// Calculate IDF (Inverse Document Frequency)
			df := s.docFreq[term]
			idf := math.Log(1.0 + (float64(s.totalDocs)-float64(df)+0.5)/(float64(df)+0.5))

			// Calculate BM25 score for this term
			// BM25 formula: IDF * (tf * (k1 + 1)) / (tf + k1 * (1 - b + b * (docLen / avgDocLen)))
			numerator := float64(tf) * (s.params.K1 + 1.0)
			denominator := float64(tf) + s.params.K1*(1.0-s.params.B+s.params.B*(docLen/s.avgDocLen))
			score += idf * (numerator / denominator)
		}

		results[i] = BM25Document{
			ID:    s.docIDs[i],
			Score: score,
		}
	}

	return results
}

// tokenize splits text into tokens (words) for BM25 processing
func tokenize(text string) []string {
	// Convert to lowercase
	text = strings.ToLower(text)

	// Split by whitespace and punctuation
	var tokens []string
	var currentToken strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			currentToken.WriteRune(r)
		} else {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
		}
	}

	// Add last token if exists
	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	return tokens
}

// NormalizeBM25Scores normalizes BM25 scores to 0-1 range
func NormalizeBM25Scores(docs []BM25Document) []BM25Document {
	if len(docs) == 0 {
		return docs
	}

	// Find max score
	maxScore := 0.0
	for _, doc := range docs {
		if doc.Score > maxScore {
			maxScore = doc.Score
		}
	}

	// Avoid division by zero
	if maxScore == 0 {
		return docs
	}

	// Normalize scores
	normalized := make([]BM25Document, len(docs))
	for i, doc := range docs {
		normalized[i] = BM25Document{
			ID:    doc.ID,
			Score: doc.Score / maxScore,
		}
	}

	return normalized
}
