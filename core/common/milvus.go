package common

import (
	"context"
	"fmt"
	"strings"

	milvusModel "github.com/Malowking/kbgo/internal/model/milvus"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/milvus-io/milvus/client/v2/entity"
	"github.com/milvus-io/milvus/client/v2/index"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
)

func CreateCollection(ctx context.Context, MilvusClient *milvusclient.Client, CollectionName string) error {
	// 使用标准 text collection schema
	Schema := &entity.Schema{
		CollectionName: CollectionName,
		Description:    "存储文档分片及其向量",
		AutoID:         false,
		Fields:         milvusModel.GetStandardCollectionFields(),
	}

	// 创建文档片段集合，并设置vector为索引
	err := MilvusClient.CreateCollection(ctx, milvusclient.NewCreateCollectionOption(CollectionName, Schema).WithIndexOptions(
		milvusclient.NewCreateIndexOption(CollectionName, "vector", index.NewHNSWIndex(entity.L2, 64, 128))))
	if err != nil {
		fmt.Printf("failed to create Milvus collection: %v\n", err)
		return err
	}

	fmt.Printf("Collection '%s' created and index built\n", CollectionName)
	return err
}

func CollectionIfNotExists(ctx context.Context, MilvusClient *milvusclient.Client, CollectionName string) (bool, error) {
	// 1. 检查集合是否存在
	has, err := MilvusClient.HasCollection(ctx, milvusclient.NewHasCollectionOption(CollectionName))
	if err != nil {
		return false, fmt.Errorf("failed to check if collection exists: %w", err)
	}
	if has {
		return false, nil // 已存在，无需创建
	}
	return true, nil
}

func containsDatabase(dbNames []string, DatabaseName string) bool {
	for _, name := range dbNames {
		if strings.EqualFold(name, DatabaseName) {
			return true
		}
	}
	return false
}

func CreateDatabaseIfNotExists(ctx context.Context, MilvusClient *milvusclient.Client, DatabaseName string) error {
	// 根据err判断database是否存在
	dbNames, err := MilvusClient.ListDatabase(ctx, milvusclient.NewListDatabaseOption())
	if err != nil {
		return err
	}
	var isExist bool
	isExist = containsDatabase(dbNames, DatabaseName)
	if !isExist {
		//如果database不存在则创建
		err = MilvusClient.CreateDatabase(ctx, milvusclient.NewCreateDatabaseOption(DatabaseName))
		if err != nil {
			return err
		}
	}
	// 如果database存在则跳过创建
	g.Log().Infof(ctx, "Database '%s' already exists, skipping creation.", DatabaseName)
	return nil
}

// DeleteCollection deletes a Milvus collection
func DeleteCollection(ctx context.Context, MilvusClient *milvusclient.Client, CollectionName string) error {
	err := MilvusClient.DropCollection(ctx, milvusclient.NewDropCollectionOption(CollectionName))
	if err != nil {
		return err
	}
	g.Log().Infof(ctx, "Collection '%s' is delete......", CollectionName)
	return nil
}

// DeleteMilvusDocument deletes all chunks of a document from Milvus collection by document_id
// This will delete all chunks that belong to the specified document
func DeleteMilvusDocument(ctx context.Context, MilvusClient *milvusclient.Client, collectionName string, documentID string) error {
	// Build filter expression to match document_id
	filterExpr := fmt.Sprintf(`document_id == "%s"`, documentID)

	g.Log().Infof(ctx, "Attempting to delete all chunks of document %s from collection %s with filter: %s", documentID, collectionName, filterExpr)
	// Create delete option with filter expression
	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filterExpr)

	// Execute delete operation
	result, err := MilvusClient.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("failed to delete document %s from collection %s: %w", documentID, collectionName, err)
	}

	g.Log().Infof(ctx, "Delete operation completed for document %s, affected rows: %d", documentID, result.DeleteCount)

	// Check if any rows were actually deleted
	if result.DeleteCount == 0 {
		g.Log().Infof(ctx, "Warning: No chunks were deleted for document_id=%s in collection %s", documentID, collectionName)
	}

	return nil
}

// DeleteMilvusChunk deletes a single chunk from Milvus collection by chunk id (primary key)
// This will only delete the specific chunk with the given id
func DeleteMilvusChunk(ctx context.Context, MilvusClient *milvusclient.Client, collectionName string, chunkID string) error {
	// Build filter expression to match the primary key id
	filterExpr := fmt.Sprintf(`id == "%s"`, chunkID)

	g.Log().Infof(ctx, "Attempting to delete chunk %s from collection %s with filter: %s", chunkID, collectionName, filterExpr)

	// Create delete option with filter expression
	deleteOpt := milvusclient.NewDeleteOption(collectionName).WithExpr(filterExpr)

	// Execute delete operation
	result, err := MilvusClient.Delete(ctx, deleteOpt)
	if err != nil {
		return fmt.Errorf("failed to delete chunk (id=%s) from collection %s: %w", chunkID, collectionName, err)
	}

	g.Log().Infof(ctx, "Delete operation completed for chunk id=%s, affected rows: %d", chunkID, result.DeleteCount)

	// Check if any rows were actually deleted
	if result.DeleteCount == 0 {
		g.Log().Infof(ctx, "Warning: No chunk was deleted for id=%s in collection %s", chunkID, collectionName)
	}

	return nil
}
