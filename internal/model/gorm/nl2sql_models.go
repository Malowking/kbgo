package gorm

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// NL2SQLDataSource 数据源表
type NL2SQLDataSource struct {
	ID       string         `gorm:"type:char(32);primaryKey" json:"id"` // 无连字符UUID
	Name     string         `gorm:"size:255;not null" json:"name"`
	Type     string         `gorm:"size:50;not null" json:"type"`      // 'csv', 'excel', 'jdbc'
	DBType   string         `gorm:"size:50" json:"db_type"`            // 'postgresql' (for JDBC)
	Config   datatypes.JSON `gorm:"type:jsonb;not null" json:"config"` // 连接配置（加密存储）
	ReadOnly bool           `gorm:"default:true" json:"read_only"`
	Status   string         `gorm:"size:50;default:'active'" json:"status"` // 'active', 'pending_confirmation', 'inactive'

	// 模型相关字段
	EmbeddingModelID string `gorm:"size:100;not null" json:"embedding_model_id"` // Embedding模型ID（用于Schema向量化）
	VectorDatabase   string `gorm:"size:100;not null" json:"vector_database"`    // 向量数据库名称（用于存储Schema向量）

	CreateTime *time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime *time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
	CreatedBy  *string    `gorm:"size:50;not null" json:"created_by"` // 外键指向 users 表
}

// BeforeCreate 创建前自动生成无连字符UUID
func (ds *NL2SQLDataSource) BeforeCreate(tx *gorm.DB) error {
	if ds.ID == "" {
		ds.ID = strings.ReplaceAll(uuid.New().String(), "-", "")
	}
	return nil
}

// TableName specifies table name
func (NL2SQLDataSource) TableName() string {
	return "nl2sql_datasources"
}

// NL2SQLMetric 指标表
type NL2SQLMetric struct {
	ID             string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DatasourceID   string         `gorm:"type:char(32);not null;index" json:"datasource_id"`     // 无连字符UUID，关联 DataSource
	MetricCode     string         `gorm:"column:metric_id;size:100;not null" json:"metric_code"` // 'metric_gmv' - 业务唯一标识符
	Name           string         `gorm:"size:255;not null" json:"name"`                         // 'GMV'
	Description    string         `gorm:"type:text" json:"description"`
	Formula        string         `gorm:"type:text;not null" json:"formula"` // 'SUM(orders.amount)'
	DefaultFilters datatypes.JSON `gorm:"type:jsonb" json:"default_filters"` // {"orders.status": "paid"}
	TimeColumn     string         `gorm:"size:255" json:"time_column"`       // 'orders.created_at'
	CreateTime     *time.Time     `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime     *time.Time     `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

// TableName specifies table name
func (NL2SQLMetric) TableName() string {
	return "nl2sql_metrics"
}

// NL2SQLTable 表元数据（L2）
type NL2SQLTable struct {
	ID               string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DatasourceID     string         `gorm:"type:char(32);not null;index" json:"datasource_id"` // 无连字符UUID，关联 DataSource
	Name             string         `gorm:"column:table_name;size:255;not null" json:"table_name"`
	DisplayName      string         `gorm:"size:255" json:"display_name"`
	Description      string         `gorm:"type:text" json:"description"`
	RowCountEstimate int64          `json:"row_count_estimate"`
	PrimaryKey       string         `gorm:"size:255" json:"primary_key"`
	TimeColumn       string         `gorm:"size:255" json:"time_column"`
	FilePath         string         `gorm:"size:500" json:"file_path"`        // CSV/Excel文件路径（仅CSV/Excel数据源使用）
	Parsed           bool           `gorm:"default:false" json:"parsed"`      // 表是否已解析
	UsagePatterns    datatypes.JSON `gorm:"type:jsonb" json:"usage_patterns"` // ['统计订单数量', '统计GMV']
	CreateTime       *time.Time     `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime       *time.Time     `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

// NL2SQLColumn 字段元数据（L3）
type NL2SQLColumn struct {
	ID            string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TableID       string         `gorm:"type:uuid;not null" json:"table_id"`
	ColumnName    string         `gorm:"size:255;not null" json:"column_name"`
	DisplayName   string         `gorm:"size:255" json:"display_name"`
	DataType      string         `gorm:"size:100;not null" json:"data_type"`
	Nullable      bool           `gorm:"default:true" json:"nullable"`
	Description   string         `gorm:"type:text" json:"description"`
	SemanticType  string         `gorm:"size:50" json:"semantic_type"` // 'id', 'currency', 'time', 'category'
	Unit          string         `gorm:"size:50" json:"unit"`          // 'CNY', 'USD'
	Examples      datatypes.JSON `gorm:"type:jsonb" json:"examples"`
	Enums         datatypes.JSON `gorm:"type:jsonb" json:"enums"`
	CommonFilters datatypes.JSON `gorm:"type:jsonb" json:"common_filters"`
	CreateTime    *time.Time     `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime    *time.Time     `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

// TableName specifies table name
func (NL2SQLColumn) TableName() string {
	return "nl2sql_columns"
}

// NL2SQLRelation 关系表（L4）
type NL2SQLRelation struct {
	ID           string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DatasourceID string     `gorm:"type:char(32);not null;index" json:"datasource_id"` // 无连字符UUID，关联 DataSource
	RelationID   string     `gorm:"size:100;not null" json:"relation_id"`
	FromTableID  string     `gorm:"type:uuid;not null" json:"from_table_id"`
	FromColumn   string     `gorm:"size:255;not null" json:"from_column"`
	ToTableID    string     `gorm:"type:uuid;not null" json:"to_table_id"`
	ToColumn     string     `gorm:"size:255;not null" json:"to_column"`
	RelationType string     `gorm:"size:50;not null" json:"relation_type"` // 'many_to_one', 'one_to_many', 'one_to_one'
	JoinType     string     `gorm:"size:50;default:'INNER'" json:"join_type"`
	Description  string     `gorm:"type:text" json:"description"`
	CreateTime   *time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
	UpdateTime   *time.Time `gorm:"column:update_time;autoUpdateTime" json:"update_time"`
}

// TableName specifies table name
func (NL2SQLRelation) TableName() string {
	return "nl2sql_relations"
}

// NL2SQLQueryLog 查询日志表（优化版，与对话系统关联）
type NL2SQLQueryLog struct {
	ID               string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	MsgID            *string        `gorm:"type:varchar(64);index" json:"msg_id"`              // 新增：关联messages表
	ConvID           *string        `gorm:"type:varchar(64);index" json:"conv_id"`             // 新增：关联conversations表
	DatasourceID     string         `gorm:"type:char(32);not null;index" json:"datasource_id"` // 无连字符UUID，关联 DataSource
	UserQuestion     string         `gorm:"type:text;not null" json:"user_question"`           // 冗余保留，方便查询
	StructuredIntent datatypes.JSON `gorm:"type:jsonb" json:"structured_intent"`               // 结构化意图
	GeneratedSQL     string         `gorm:"type:text" json:"generated_sql"`
	FinalSQL         string         `gorm:"type:text" json:"final_sql"`            // 修复后的SQL
	ExecutionStatus  string         `gorm:"size:50;index" json:"execution_status"` // 'success', 'failed', 'timeout'，添加索引
	ExecutionTimeMs  int            `gorm:"index" json:"execution_time_ms"`        // 添加索引，方便查询慢查询
	ErrorMessage     string         `gorm:"type:text" json:"error_message"`
	ResultRows       int            `json:"result_rows"`
	UserFeedback     string         `gorm:"size:50" json:"user_feedback"` // 'correct', 'incorrect', null
	FeedbackComment  string         `gorm:"type:text" json:"feedback_comment"`
	Extra            datatypes.JSON `gorm:"type:jsonb" json:"extra"`                                    // 扩展字段，可存储 export_file_url 等
	CreateTime       *time.Time     `gorm:"column:create_time;autoCreateTime;index" json:"create_time"` // 添加索引
	UserID           *string        `gorm:"type:uuid" json:"user_id"`
}

// TableName specifies table name
func (NL2SQLQueryLog) TableName() string {
	return "nl2sql_query_logs"
}

// NL2SQLVectorDoc 向量索引关联表
type NL2SQLVectorDoc struct {
	ID            string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DatasourceID  string     `gorm:"type:char(32);not null;index" json:"datasource_id"` // 无连字符UUID，关联 DataSource
	EntityType    string     `gorm:"size:50;not null;index" json:"entity_type"`         // 'metric', 'table', 'column', 'relation'
	EntityID      string     `gorm:"type:uuid;not null;index" json:"entity_id"`
	DocumentID    string     `gorm:"type:uuid;not null" json:"document_id"`    // 外键指向 knowledge_documents 表
	VectorContent string     `gorm:"type:text;not null" json:"vector_content"` // 用于embedding的文本
	CreateTime    *time.Time `gorm:"column:create_time;autoCreateTime" json:"create_time"`
}

// TableName specifies table name
func (NL2SQLVectorDoc) TableName() string {
	return "nl2sql_vector_docs"
}
