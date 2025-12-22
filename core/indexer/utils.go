package indexer

import (
	"context"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/gogf/gf/v2/frame/g"
)

// DeleteChunk Delete specified chunk from vector database
func (s *DocumentIndexer) DeleteChunk(ctx context.Context, collectionName string, chunkID string) error {
	err := s.VectorStore.DeleteByChunkID(ctx, collectionName, chunkID)
	if err != nil {
		return errors.Newf(errors.ErrVectorDelete, "Failed to delete chunk: %v", err)
	}

	g.Log().Infof(ctx, "Successfully deleted chunk, collection=%s, chunkID=%s", collectionName, chunkID)
	return nil
}

// DeleteDocument Delete all chunks of specified document from vector database
func (s *DocumentIndexer) DeleteDocument(ctx context.Context, collectionName string, documentID string) error {
	err := s.VectorStore.DeleteByDocumentID(ctx, collectionName, documentID)
	if err != nil {
		return errors.Newf(errors.ErrVectorDelete, "Failed to delete document: %v", err)
	}

	g.Log().Infof(ctx, "Successfully deleted document, collection=%s, documentID=%s", collectionName, documentID)
	return nil
}

// GetVectorStore Get vector store instance
func (s *DocumentIndexer) GetVectorStore() vector_store.VectorStore {
	return s.VectorStore
}
