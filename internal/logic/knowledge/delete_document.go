package knowledge

import (
	"context"

	"github.com/Malowking/kbgo/core/errors"
	"github.com/Malowking/kbgo/core/vector_store"
	"github.com/Malowking/kbgo/internal/dao"
	"github.com/gogf/gf/v2/frame/g"
)

// DeleteDocumentDataOnly deletes the chunks data for the specified document, but keeps the document record
func DeleteDocumentDataOnly(ctx context.Context, documentId string, vectorStore vector_store.VectorStore) error {
	// Begin transaction
	tx := dao.GetDB().Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get document information
	document, err := GetDocumentById(ctx, documentId)
	if err != nil {
		g.Log().Errorf(ctx, "DeleteDocumentDataOnly: GetDocumentById failed for id %s, err: %v", documentId, err)
		tx.Rollback()
		return errors.Newf(errors.ErrDocumentNotFound, "failed to get document information: %v", err)
	}

	// Check if CollectionName exists
	if document.CollectionName == "" {
		g.Log().Warningf(ctx, "DeleteDocumentDataOnly: CollectionName is empty for document id %s", documentId)
	} else {
		// Use VectorStore interface to delete all chunks of this document
		err = vectorStore.DeleteByDocumentID(ctx, document.CollectionName, documentId)
		if err != nil {
			g.Log().Errorf(ctx, "DeleteDocumentDataOnly: Vector store deleteDocument failed for documentId %s in collection %s, err: %v", documentId, document.CollectionName, err)
			tx.Rollback()
			return errors.Newf(errors.ErrVectorDelete, "failed to delete document data in vector store: %v", err)
		}
		g.Log().Infof(ctx, "DeleteDocumentDataOnly: Successfully deleted document %s from collection %s", documentId, document.CollectionName)
	}

	// Only delete chunks data, keep the document record
	err = DeleteChunksByDocumentIdWithTx(ctx, tx, documentId)
	if err != nil {
		g.Log().Errorf(ctx, "DeleteDocumentDataOnly: DeleteChunksByDocumentId failed for id %s, err: %v", documentId, err)
		tx.Rollback()
		return errors.Newf(errors.ErrDatabaseDelete, "failed to delete chunks data: %v", err)
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		g.Log().Errorf(ctx, "DeleteDocumentDataOnly: transaction commit failed, err: %v", err)
		return errors.Newf(errors.ErrInternalError, "failed to commit transaction: %v", err)
	}

	g.Log().Infof(ctx, "DeleteDocumentDataOnly: Successfully deleted chunks data for document id %s", documentId)
	return nil
}
