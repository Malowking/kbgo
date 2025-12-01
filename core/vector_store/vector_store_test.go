package vector_store

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Malowking/kbgo/core/common"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVectorStoreInterface 测试两种数据库是否都实现了接口
func TestVectorStoreInterface(t *testing.T) {
	t.Run("Milvus实现VectorStore接口", func(t *testing.T) {
		var _ VectorStore = (*MilvusStore)(nil)
	})

	t.Run("PostgreSQL实现VectorStore接口", func(t *testing.T) {
		var _ VectorStore = (*PostgresStore)(nil)
	})
}

// TestFactoryCreation 测试工厂函数
func TestFactoryCreation(t *testing.T) {
	t.Run("创建Milvus存储", func(t *testing.T) {
		// 这里需要实际的 Milvus 客户端，测试会跳过如果服务未运行
		config := &VectorStoreConfig{
			Type:     VectorStoreTypeMilvus,
			Database: "test",
		}

		// 没有客户端应该失败
		store, err := NewVectorStore(config)
		assert.Error(t, err)
		assert.Nil(t, store)
	})

	t.Run("创建PostgreSQL存储", func(t *testing.T) {
		ctx := context.Background()
		config := &VectorStoreConfig{
			Type:     VectorStoreTypePostgreSQL,
			Database: "test",
		}

		// 没有客户端应该失败
		store, err := NewVectorStore(config)
		assert.Error(t, err)
		assert.Nil(t, store)
		_ = ctx
	})

	t.Run("不支持的类型", func(t *testing.T) {
		config := &VectorStoreConfig{
			Type:     "unsupported",
			Database: "test",
		}

		store, err := NewVectorStore(config)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "unsupported vector store type")
	})
}

// compareVectorStorePerformance 对比两种向量数据库的性能
func compareVectorStorePerformance(t *testing.T, milvusStore, postgresStore VectorStore) {
	if milvusStore == nil || postgresStore == nil {
		t.Skip("需要 Milvus 和 PostgreSQL 都可用")
	}

	ctx := context.Background()
	documentID := uuid.New().String()
	knowledgeID := uuid.New().String()
	ctx = context.WithValue(ctx, common.DocumentId, documentID)
	ctx = context.WithValue(ctx, common.KnowledgeId, knowledgeID)

	milvusCollection := "perf_test_milvus_" + uuid.New().String()[:8]
	postgresCollection := "perf_test_postgres_" + uuid.New().String()[:8]

	// 创建集合
	err := milvusStore.CreateCollection(ctx, milvusCollection)
	require.NoError(t, err)
	defer milvusStore.DeleteCollection(ctx, milvusCollection)

	err = postgresStore.CreateCollection(ctx, postgresCollection)
	require.NoError(t, err)
	defer postgresStore.DeleteCollection(ctx, postgresCollection)

	// 准备测试数据
	numVectors := 100
	chunks := make([]*schema.Document, numVectors)
	vectors := make([][]float64, numVectors)

	for i := 0; i < numVectors; i++ {
		chunks[i] = &schema.Document{
			ID:      uuid.New().String(),
			Content: fmt.Sprintf("测试文档 %d", i),
			MetaData: map[string]any{
				"index": i,
			},
		}
		vectors[i] = make([]float64, 1024)
		for j := range vectors[i] {
			vectors[i][j] = float64(i*j) * 0.001
		}
	}

	t.Run("插入性能对比", func(t *testing.T) {
		// Milvus 插入
		startMilvus := time.Now()
		_, err := milvusStore.InsertVectors(ctx, milvusCollection, chunks, vectors)
		milvusDuration := time.Since(startMilvus)
		assert.NoError(t, err)

		// PostgreSQL 插入
		startPostgres := time.Now()
		_, err = postgresStore.InsertVectors(ctx, postgresCollection, chunks, vectors)
		postgresDuration := time.Since(startPostgres)
		assert.NoError(t, err)

		t.Logf("插入 %d 个向量:", numVectors)
		t.Logf("  Milvus:     %v", milvusDuration)
		t.Logf("  PostgreSQL: %v", postgresDuration)
		t.Logf("  比率: %.2fx", float64(postgresDuration)/float64(milvusDuration))
	})

	t.Run("删除性能对比", func(t *testing.T) {
		// Milvus 删除
		startMilvus := time.Now()
		err := milvusStore.DeleteByDocumentID(ctx, milvusCollection, documentID)
		milvusDuration := time.Since(startMilvus)
		assert.NoError(t, err)

		// PostgreSQL 删除
		startPostgres := time.Now()
		err = postgresStore.DeleteByDocumentID(ctx, postgresCollection, documentID)
		postgresDuration := time.Since(startPostgres)
		assert.NoError(t, err)

		t.Logf("删除文档向量:")
		t.Logf("  Milvus:     %v", milvusDuration)
		t.Logf("  PostgreSQL: %v", postgresDuration)
		t.Logf("  比率: %.2fx", float64(postgresDuration)/float64(milvusDuration))
	})
}

// TestVectorStoreComparison 对比测试（需要两个数据库都运行）
func TestVectorStoreComparison(t *testing.T) {
	t.Skip("需要手动启用此测试，并确保 Milvus 和 PostgreSQL 都在运行")

	// 这里需要实际初始化两个存储
	// var milvusStore VectorStore
	// var postgresStore VectorStore
	// compareVectorStorePerformance(t, milvusStore, postgresStore)
}

// TestConcurrentOperations 并发操作测试
func TestConcurrentOperations(t *testing.T) {
	t.Run("PostgreSQL并发插入", func(t *testing.T) {
		// 测试 PostgreSQL 的事务隔离
		t.Skip("需要实际的 PostgreSQL 连接")
	})

	t.Run("Milvus并发插入", func(t *testing.T) {
		// 测试 Milvus 的并发性能
		t.Skip("需要实际的 Milvus 连接")
	})
}

// generateTestVectors 生成测试向量
func generateTestVectors(count, dim int) [][]float64 {
	vectors := make([][]float64, count)
	for i := 0; i < count; i++ {
		vectors[i] = make([]float64, dim)
		for j := 0; j < dim; j++ {
			vectors[i][j] = float64(i*j) * 0.001
		}
	}
	return vectors
}

// generateTestDocuments 生成测试文档
func generateTestDocuments(count int) []*schema.Document {
	docs := make([]*schema.Document, count)
	for i := 0; i < count; i++ {
		docs[i] = &schema.Document{
			ID:      uuid.New().String(),
			Content: fmt.Sprintf("测试文档 %d: 这是一个用于测试向量数据库性能的文档", i),
			MetaData: map[string]any{
				"index":  i,
				"source": "test",
				"type":   "performance_test",
			},
		}
	}
	return docs
}

// TestLargeScaleOperations 大规模操作测试
func TestLargeScaleOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过大规模测试")
	}

	t.Run("插入1000个向量", func(t *testing.T) {
		t.Skip("需要实际的数据库连接")
		// 测试大规模插入性能
	})

	t.Run("插入10000个向量", func(t *testing.T) {
		t.Skip("需要实际的数据库连接")
		// 测试更大规模的插入
	})
}

// TestDataConsistency 数据一致性测试
func TestDataConsistency(t *testing.T) {
	t.Run("插入后立即查询", func(t *testing.T) {
		t.Skip("需要实际的数据库连接")
		// 测试数据一致性
	})

	t.Run("删除后验证", func(t *testing.T) {
		t.Skip("需要实际的数据库连接")
		// 验证删除操作的完整性
	})

	t.Run("更新元数据", func(t *testing.T) {
		t.Skip("需要实际的数据库连接")
		// 测试元数据更新
	})
}

// TestErrorHandling 错误处理测试
func TestErrorHandling(t *testing.T) {
	t.Run("连接断开恢复", func(t *testing.T) {
		t.Skip("需要手动测试连接恢复")
	})

	t.Run("无效数据处理", func(t *testing.T) {
		t.Skip("需要实际的数据库连接")
	})

	t.Run("超时处理", func(t *testing.T) {
		t.Skip("需要实际的数据库连接")
	})
}

// ExampleVectorStore_InsertVectors 插入向量示例
func ExampleVectorStore_InsertVectors() {
	// 这是一个示例，展示如何使用向量存储
	ctx := context.Background()

	// 假设已经初始化了 vectorStore
	// vectorStore := ...

	// 准备文档
	chunks := []*schema.Document{
		{
			ID:      uuid.New().String(),
			Content: "这是一个示例文档",
			MetaData: map[string]any{
				"source": "example",
			},
		},
	}

	// 准备向量（1024维）
	vectors := [][]float64{
		make([]float64, 1024),
	}

	// 设置上下文
	documentID := uuid.New().String()
	knowledgeID := uuid.New().String()
	ctx = context.WithValue(ctx, common.DocumentId, documentID)
	ctx = context.WithValue(ctx, common.KnowledgeId, knowledgeID)

	// 插入向量
	// ids, err := vectorStore.InsertVectors(ctx, "my_collection", chunks, vectors)
	// if err != nil {
	//     log.Fatal(err)
	// }
	// fmt.Printf("插入成功，IDs: %v\n", ids)

	_ = ctx
	_ = chunks
	_ = vectors
}

// ExampleVectorStore_DeleteByDocumentID 删除文档示例
func ExampleVectorStore_DeleteByDocumentID() {
	// 这是一个示例，展示如何删除文档的所有向量
	ctx := context.Background()

	// 假设已经初始化了 vectorStore
	// vectorStore := ...

	documentID := "550e8400-e29b-41d4-a716-446655440000"

	// 删除文档的所有向量
	// err := vectorStore.DeleteByDocumentID(ctx, "my_collection", documentID)
	// if err != nil {
	//     log.Fatal(err)
	// }
	// fmt.Println("删除成功")

	_ = ctx
	_ = documentID
}
