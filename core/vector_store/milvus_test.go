package vector_store

import (
	"context"
	"testing"

	"github.com/Malowking/kbgo/core/common"
	"github.com/Malowking/kbgo/pkg/schema"
	"github.com/google/uuid"
	"github.com/milvus-io/milvus/client/v2/milvusclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// convertToFloat32 converts [][]float64 to [][]float32
func convertToFloat32(vectors [][]float64) [][]float32 {
	result := make([][]float32, len(vectors))
	for i, vec := range vectors {
		result[i] = make([]float32, len(vec))
		for j, val := range vec {
			result[i][j] = float32(val)
		}
	}
	return result
}

// TestMilvusStoreCreation 测试 Milvus 存储实例创建
func TestMilvusStoreCreation(t *testing.T) {
	t.Run("创建成功", func(t *testing.T) {
		ctx := context.Background()
		client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
			Address: "localhost:19530",
			DBName:  "test",
		})
		if err != nil {
			t.Skip("Milvus 未运行，跳过测试")
		}

		config := &VectorStoreConfig{
			Type:     VectorStoreTypeMilvus,
			Client:   client,
			Database: "test",
		}

		store, err := NewMilvusStore(config)
		require.NoError(t, err)
		assert.NotNil(t, store)
	})

	t.Run("配置为nil", func(t *testing.T) {
		store, err := NewMilvusStore(nil)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("客户端类型错误", func(t *testing.T) {
		config := &VectorStoreConfig{
			Type:     VectorStoreTypeMilvus,
			Client:   "invalid_client", // 错误的类型
			Database: "test",
		}

		store, err := NewMilvusStore(config)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "must be *milvusclient.Client")
	})

	t.Run("数据库名为空", func(t *testing.T) {
		ctx := context.Background()
		client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
			Address: "localhost:19530",
		})
		if err != nil {
			t.Skip("Milvus 未运行，跳过测试")
		}

		config := &VectorStoreConfig{
			Type:     VectorStoreTypeMilvus,
			Client:   client,
			Database: "", // 空数据库名
		}

		store, err := NewMilvusStore(config)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "database name cannot be empty")
	})
}

// TestMilvusCollectionOperations 测试 Milvus 集合操作
func TestMilvusCollectionOperations(t *testing.T) {
	ctx := context.Background()
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: "localhost:19530",
		DBName:  "test",
	})
	if err != nil {
		t.Skip("Milvus 未运行，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypeMilvus,
		Client:   client,
		Database: "test",
	}

	store, err := NewMilvusStore(config)
	require.NoError(t, err)

	testCollectionName := "test_collection_" + uuid.New().String()[:8]

	t.Run("创建集合", func(t *testing.T) {
		err := store.CreateCollection(ctx, testCollectionName)
		assert.NoError(t, err)
	})

	t.Run("检查集合存在", func(t *testing.T) {
		exists, err := store.CollectionExists(ctx, testCollectionName)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("检查不存在的集合", func(t *testing.T) {
		exists, err := store.CollectionExists(ctx, "non_existent_collection")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("删除集合", func(t *testing.T) {
		err := store.DeleteCollection(ctx, testCollectionName)
		assert.NoError(t, err)

		// 验证已删除
		exists, err := store.CollectionExists(ctx, testCollectionName)
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

// TestMilvusVectorOperations 测试 Milvus 向量操作
func TestMilvusVectorOperations(t *testing.T) {
	ctx := context.Background()
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: "localhost:19530",
		DBName:  "test",
	})
	if err != nil {
		t.Skip("Milvus 未运行，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypeMilvus,
		Client:   client,
		Database: "test",
	}

	store, err := NewMilvusStore(config)
	require.NoError(t, err)

	testCollectionName := "test_vectors_" + uuid.New().String()[:8]
	documentID := uuid.New().String()
	knowledgeID := uuid.New().String()

	// 创建测试集合
	err = store.CreateCollection(ctx, testCollectionName)
	require.NoError(t, err)

	// 清理
	defer store.DeleteCollection(ctx, testCollectionName)

	t.Run("插入向量", func(t *testing.T) {
		// 准备测试数据
		chunks := []*schema.Document{
			{
				ID:      uuid.New().String(),
				Content: "这是第一个测试文档",
				MetaData: map[string]any{
					"source": "test",
				},
			},
			{
				ID:      uuid.New().String(),
				Content: "这是第二个测试文档",
				MetaData: map[string]any{
					"source": "test",
				},
			},
		}

		// 生成测试向量（1024维）
		vectors := make([][]float64, 2)
		for i := range vectors {
			vectors[i] = make([]float64, 1024)
			for j := range vectors[i] {
				vectors[i][j] = float64(i*j) * 0.01
			}
		}

		// 设置上下文
		ctx = context.WithValue(ctx, common.DocumentId, documentID)
		ctx = context.WithValue(ctx, common.KnowledgeId, knowledgeID)

		ids, err := store.InsertVectors(ctx, testCollectionName, chunks, convertToFloat32(vectors))
		assert.NoError(t, err)
		assert.Equal(t, 2, len(ids))
		assert.Equal(t, chunks[0].ID, ids[0])
		assert.Equal(t, chunks[1].ID, ids[1])
	})

	t.Run("插入向量数量不匹配", func(t *testing.T) {
		chunks := []*schema.Document{
			{ID: uuid.New().String(), Content: "test"},
		}
		vectors := [][]float64{{1.0}, {2.0}} // 数量不匹配

		ctx = context.WithValue(ctx, common.DocumentId, documentID)
		ctx = context.WithValue(ctx, common.KnowledgeId, knowledgeID)

		ids, err := store.InsertVectors(ctx, testCollectionName, chunks, convertToFloat32(vectors))
		assert.Error(t, err)
		assert.Nil(t, ids)
		assert.Contains(t, err.Error(), "length mismatch")
	})

	t.Run("删除文档向量", func(t *testing.T) {
		err := store.DeleteByDocumentID(ctx, testCollectionName, documentID)
		assert.NoError(t, err)
	})

	t.Run("删除单个chunk", func(t *testing.T) {
		chunkID := uuid.New().String()
		err := store.DeleteByChunkID(ctx, testCollectionName, chunkID)
		// 即使chunk不存在也应该成功（返回0行）
		assert.NoError(t, err)
	})

	t.Run("删除无效的UUID格式", func(t *testing.T) {
		err := store.DeleteByChunkID(ctx, testCollectionName, "invalid-uuid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chunk ID format")
	})
}

// TestMilvusHelperFunctions 测试 Milvus 辅助函数
func TestMilvusHelperFunctions(t *testing.T) {
	t.Run("truncateString", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			maxLen   int
			expected string
		}{
			{"短字符串", "hello", 10, "hello"},
			{"长字符串", "hello world", 5, "hello"},
			{"空字符串", "", 10, ""},
			{"恰好最大长度", "hello", 5, "hello"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := truncateString(tt.input, tt.maxLen)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("float64ToFloat32", func(t *testing.T) {
		input := []float64{1.0, 2.5, 3.14159}
		result := float64ToFloat32(input)

		assert.Equal(t, len(input), len(result))
		for i := range input {
			assert.InDelta(t, input[i], float64(result[i]), 0.0001)
		}
	})

	t.Run("marshalMetadata", func(t *testing.T) {
		tests := []struct {
			name     string
			input    map[string]any
			hasError bool
		}{
			{"空metadata", map[string]any{}, false},
			{"普通metadata", map[string]any{"key": "value", "num": 123}, false},
			{"嵌套metadata", map[string]any{"nested": map[string]any{"key": "value"}}, false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result, err := marshalMetadata(tt.input)
				if tt.hasError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.NotNil(t, result)
				}
			})
		}
	})
}

// TestMilvusGetClient 测试获取客户端
func TestMilvusGetClient(t *testing.T) {
	ctx := context.Background()
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: "localhost:19530",
		DBName:  "test",
	})
	if err != nil {
		t.Skip("Milvus 未运行，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypeMilvus,
		Client:   client,
		Database: "test",
	}

	store, err := NewMilvusStore(config)
	require.NoError(t, err)

	t.Run("GetClient返回interface{}", func(t *testing.T) {
		clientInterface := store.GetClient()
		assert.NotNil(t, clientInterface)
	})

	t.Run("GetMilvusClient返回具体类型", func(t *testing.T) {
		milvusStore, ok := store.(*MilvusStore)
		require.True(t, ok)

		milvusClient := milvusStore.GetMilvusClient()
		assert.NotNil(t, milvusClient)
		assert.IsType(t, &milvusclient.Client{}, milvusClient)
	})
}

// BenchmarkMilvusInsertVectors 性能测试：插入向量
func BenchmarkMilvusInsertVectors(b *testing.B) {
	ctx := context.Background()
	client, err := milvusclient.New(ctx, &milvusclient.ClientConfig{
		Address: "localhost:19530",
		DBName:  "test",
	})
	if err != nil {
		b.Skip("Milvus 未运行，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypeMilvus,
		Client:   client,
		Database: "test",
	}

	store, err := NewMilvusStore(config)
	if err != nil {
		b.Fatal(err)
	}

	testCollectionName := "bench_collection"
	store.CreateCollection(ctx, testCollectionName)
	defer store.DeleteCollection(ctx, testCollectionName)

	// 准备测试数据
	chunks := []*schema.Document{
		{ID: uuid.New().String(), Content: "test document"},
	}
	vectors := [][]float64{make([]float64, 1024)}
	for i := range vectors[0] {
		vectors[0][i] = float64(i) * 0.01
	}

	documentID := uuid.New().String()
	knowledgeID := uuid.New().String()
	ctx = context.WithValue(ctx, common.DocumentId, documentID)
	ctx = context.WithValue(ctx, common.KnowledgeId, knowledgeID)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		chunks[0].ID = uuid.New().String()
		_, err := store.InsertVectors(ctx, testCollectionName, chunks, convertToFloat32(vectors))
		if err != nil {
			b.Fatal(err)
		}
	}
}
