package vector_store

import (
	"context"
	"testing"

	"github.com/Malowking/kbgo/core/common"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPostgresStoreCreation 测试 PostgreSQL 存储实例创建
func TestPostgresStoreCreation(t *testing.T) {
	t.Run("创建成功", func(t *testing.T) {
		ctx := context.Background()
		connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
		pool, err := pgxpool.New(ctx, connStr)
		if err != nil {
			t.Skip("PostgreSQL 未运行，跳过测试")
		}
		defer pool.Close()

		config := &VectorStoreConfig{
			Type:     VectorStoreTypePostgreSQL,
			Client:   pool,
			Database: "test_kbgo",
		}

		store, err := NewPostgresStore(config)
		require.NoError(t, err)
		assert.NotNil(t, store)
	})

	t.Run("配置为nil", func(t *testing.T) {
		store, err := NewPostgresStore(nil)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "config cannot be nil")
	})

	t.Run("客户端类型错误", func(t *testing.T) {
		config := &VectorStoreConfig{
			Type:     VectorStoreTypePostgreSQL,
			Client:   "invalid_client", // 错误的类型
			Database: "test",
		}

		store, err := NewPostgresStore(config)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "must be *pgxpool.Pool")
	})

	t.Run("数据库名为空", func(t *testing.T) {
		ctx := context.Background()
		connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
		pool, err := pgxpool.New(ctx, connStr)
		if err != nil {
			t.Skip("PostgreSQL 未运行，跳过测试")
		}
		defer pool.Close()

		config := &VectorStoreConfig{
			Type:     VectorStoreTypePostgreSQL,
			Client:   pool,
			Database: "", // 空数据库名
		}

		store, err := NewPostgresStore(config)
		assert.Error(t, err)
		assert.Nil(t, store)
		assert.Contains(t, err.Error(), "database name cannot be empty")
	})
}

// TestPostgresCreateDatabaseIfNotExists 测试创建数据库和扩展
func TestPostgresCreateDatabaseIfNotExists(t *testing.T) {
	ctx := context.Background()
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skip("PostgreSQL 未运行，跳过测试")
	}
	defer pool.Close()

	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: "test_kbgo",
	}

	store, err := NewPostgresStore(config)
	require.NoError(t, err)

	t.Run("创建pgvector扩展", func(t *testing.T) {
		err := store.CreateDatabaseIfNotExists(ctx)
		// 如果扩展未安装，会失败
		if err != nil {
			t.Logf("pgvector 扩展可能未安装: %v", err)
		}
	})
}

// TestPostgresCollectionOperations 测试 PostgreSQL 集合（表）操作
func TestPostgresCollectionOperations(t *testing.T) {
	ctx := context.Background()
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skip("PostgreSQL 未运行，跳过测试")
	}
	defer pool.Close()

	// 确保 pgvector 扩展已安装
	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		t.Skip("pgvector 扩展未安装，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: "test_kbgo",
	}

	store, err := NewPostgresStore(config)
	require.NoError(t, err)

	testTableName := "test_table_" + uuid.New().String()[:8]

	t.Run("创建集合（表）", func(t *testing.T) {
		err := store.CreateCollection(ctx, testTableName)
		assert.NoError(t, err)
	})

	t.Run("检查集合存在", func(t *testing.T) {
		exists, err := store.CollectionExists(ctx, testTableName)
		assert.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("检查不存在的集合", func(t *testing.T) {
		exists, err := store.CollectionExists(ctx, "non_existent_table")
		assert.NoError(t, err)
		assert.False(t, exists)
	})

	t.Run("重复创建集合", func(t *testing.T) {
		// 使用 IF NOT EXISTS，不应报错
		err := store.CreateCollection(ctx, testTableName)
		assert.NoError(t, err)
	})

	t.Run("删除集合（表）", func(t *testing.T) {
		err := store.DeleteCollection(ctx, testTableName)
		assert.NoError(t, err)

		// 验证已删除
		exists, err := store.CollectionExists(ctx, testTableName)
		assert.NoError(t, err)
		assert.False(t, exists)
	})
}

// TestPostgresVectorOperations 测试 PostgreSQL 向量操作
func TestPostgresVectorOperations(t *testing.T) {
	ctx := context.Background()
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skip("PostgreSQL 未运行，跳过测试")
	}
	defer pool.Close()

	// 确保 pgvector 扩展已安装
	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		t.Skip("pgvector 扩展未安装，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: "test_kbgo",
	}

	store, err := NewPostgresStore(config)
	require.NoError(t, err)

	testTableName := "test_vectors_" + uuid.New().String()[:8]
	documentID := uuid.New().String()
	knowledgeID := uuid.New().String()

	// 创建测试表
	err = store.CreateCollection(ctx, testTableName)
	require.NoError(t, err)

	// 清理
	defer store.DeleteCollection(ctx, testTableName)

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

		ids, err := store.InsertVectors(ctx, testTableName, chunks, vectors)
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

		ids, err := store.InsertVectors(ctx, testTableName, chunks, vectors)
		assert.Error(t, err)
		assert.Nil(t, ids)
		assert.Contains(t, err.Error(), "length mismatch")
	})

	t.Run("缺少document_id上下文", func(t *testing.T) {
		chunks := []*schema.Document{
			{ID: uuid.New().String(), Content: "test"},
		}
		vectors := [][]float64{make([]float64, 1024)}

		// 不设置 DocumentId
		ctxWithoutDoc := context.WithValue(context.Background(), common.KnowledgeId, knowledgeID)

		ids, err := store.InsertVectors(ctxWithoutDoc, testTableName, chunks, vectors)
		assert.Error(t, err)
		assert.Nil(t, ids)
		assert.Contains(t, err.Error(), "document_id not found")
	})

	t.Run("删除文档向量", func(t *testing.T) {
		err := store.DeleteByDocumentID(ctx, testTableName, documentID)
		assert.NoError(t, err)
	})

	t.Run("删除单个chunk", func(t *testing.T) {
		chunkID := uuid.New().String()
		err := store.DeleteByChunkID(ctx, testTableName, chunkID)
		// 即使chunk不存在也应该成功（返回0行）
		assert.NoError(t, err)
	})

	t.Run("删除无效的UUID格式", func(t *testing.T) {
		err := store.DeleteByChunkID(ctx, testTableName, "invalid-uuid")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid chunk ID format")
	})
}

// TestPostgresHelperFunctions 测试 PostgreSQL 辅助函数
func TestPostgresHelperFunctions(t *testing.T) {
	ctx := context.Background()
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skip("PostgreSQL 未运行，跳过测试")
	}
	defer pool.Close()

	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: "test_kbgo",
	}

	store, err := NewPostgresStore(config)
	require.NoError(t, err)

	postgresStore, ok := store.(*PostgresStore)
	require.True(t, ok)

	t.Run("sanitizeTableName", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			expected string
		}{
			{"普通表名", "my_table", "my_table"},
			{"包含特殊字符", "my-table.name", "my_table_name"},
			{"包含空格", "my table", "my_table"},
			{"包含中文", "我的表", "___"},
			{"混合字符", "table_123-name", "table_123_name"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				result := postgresStore.sanitizeTableName(tt.input)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

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
				result := postgresStore.truncateString(tt.input, tt.maxLen)
				assert.Equal(t, tt.expected, result)
			})
		}
	})

	t.Run("float64ToFloat32", func(t *testing.T) {
		input := []float64{1.0, 2.5, 3.14159}
		result := postgresStore.float64ToFloat32(input)

		assert.Equal(t, len(input), len(result))
		for i := range input {
			assert.InDelta(t, input[i], float64(result[i]), 0.0001)
		}
	})
}

// TestPostgresGetClient 测试获取客户端
func TestPostgresGetClient(t *testing.T) {
	ctx := context.Background()
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skip("PostgreSQL 未运行，跳过测试")
	}
	defer pool.Close()

	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: "test_kbgo",
	}

	store, err := NewPostgresStore(config)
	require.NoError(t, err)

	t.Run("GetClient返回interface{}", func(t *testing.T) {
		clientInterface := store.GetClient()
		assert.NotNil(t, clientInterface)
	})

	t.Run("GetClient可以转换回pgxpool.Pool", func(t *testing.T) {
		clientInterface := store.GetClient()
		pool, ok := clientInterface.(*pgxpool.Pool)
		assert.True(t, ok)
		assert.NotNil(t, pool)
	})
}

// TestPostgresTransactionRollback 测试事务回滚
func TestPostgresTransactionRollback(t *testing.T) {
	ctx := context.Background()
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		t.Skip("PostgreSQL 未运行，跳过测试")
	}
	defer pool.Close()

	// 确保 pgvector 扩展已安装
	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		t.Skip("pgvector 扩展未安装，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: "test_kbgo",
	}

	store, err := NewPostgresStore(config)
	require.NoError(t, err)

	testTableName := "test_transaction_" + uuid.New().String()[:8]
	documentID := uuid.New().String()
	knowledgeID := uuid.New().String()

	// 创建测试表
	err = store.CreateCollection(ctx, testTableName)
	require.NoError(t, err)
	defer store.DeleteCollection(ctx, testTableName)

	t.Run("插入无效向量导致回滚", func(t *testing.T) {
		chunks := []*schema.Document{
			{ID: uuid.New().String(), Content: "test1"},
			{ID: uuid.New().String(), Content: "test2"},
		}

		// 创建一个维度不正确的向量（应该是1024维）
		vectors := [][]float64{
			make([]float64, 100), // 错误的维度
			make([]float64, 100),
		}

		ctx = context.WithValue(ctx, common.DocumentId, documentID)
		ctx = context.WithValue(ctx, common.KnowledgeId, knowledgeID)

		ids, err := store.InsertVectors(ctx, testTableName, chunks, vectors)
		assert.Error(t, err)
		assert.Nil(t, ids)

		// 验证没有数据被插入（事务已回滚）
		var count int
		err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM "+testTableName).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 0, count)
	})
}

// BenchmarkPostgresInsertVectors 性能测试：插入向量
func BenchmarkPostgresInsertVectors(b *testing.B) {
	ctx := context.Background()
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		b.Skip("PostgreSQL 未运行，跳过测试")
	}
	defer pool.Close()

	// 确保 pgvector 扩展已安装
	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		b.Skip("pgvector 扩展未安装，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: "test_kbgo",
	}

	store, err := NewPostgresStore(config)
	if err != nil {
		b.Fatal(err)
	}

	testTableName := "bench_table"
	store.CreateCollection(ctx, testTableName)
	defer store.DeleteCollection(ctx, testTableName)

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
		_, err := store.InsertVectors(ctx, testTableName, chunks, vectors)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPostgresVectorSearch 性能测试：向量搜索
func BenchmarkPostgresVectorSearch(b *testing.B) {
	ctx := context.Background()
	connStr := "host=localhost port=5432 user=postgres password=postgres dbname=test_kbgo sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		b.Skip("PostgreSQL 未运行，跳过测试")
	}
	defer pool.Close()

	// 确保 pgvector 扩展已安装
	_, err = pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		b.Skip("pgvector 扩展未安装，跳过测试")
	}

	config := &VectorStoreConfig{
		Type:     VectorStoreTypePostgreSQL,
		Client:   pool,
		Database: "test_kbgo",
	}

	store, err := NewPostgresStore(config)
	if err != nil {
		b.Fatal(err)
	}

	postgresStore := store.(*PostgresStore)
	testTableName := "bench_search_" + uuid.New().String()[:8]

	// 创建表并插入一些测试数据
	store.CreateCollection(ctx, testTableName)
	defer store.DeleteCollection(ctx, testTableName)

	// 插入100个测试向量
	documentID := uuid.New().String()
	knowledgeID := uuid.New().String()
	ctx = context.WithValue(ctx, common.DocumentId, documentID)
	ctx = context.WithValue(ctx, common.KnowledgeId, knowledgeID)

	for i := 0; i < 100; i++ {
		chunks := []*schema.Document{
			{ID: uuid.New().String(), Content: "test document"},
		}
		vectors := [][]float64{make([]float64, 1024)}
		for j := range vectors[0] {
			vectors[0][j] = float64(i*j) * 0.01
		}
		store.InsertVectors(ctx, testTableName, chunks, vectors)
	}

	// 准备查询向量
	queryVector := make([]float64, 1024)
	for i := range queryVector {
		queryVector[i] = float64(i) * 0.01
	}

	tableName := postgresStore.sanitizeTableName(testTableName)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 执行向量搜索
		searchSQL := `
			SELECT id, text, document_id, metadata,
			       1 - (vector <=> $1) as similarity_score
			FROM ` + tableName + `
			ORDER BY vector <=> $1
			LIMIT 10
		`
		rows, err := postgresStore.pool.Query(ctx, searchSQL, queryVector)
		if err != nil {
			b.Fatal(err)
		}
		rows.Close()
	}
}
