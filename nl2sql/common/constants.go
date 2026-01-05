package common

// 支持的数据库类型
const (
	DBTypePostgreSQL = "postgresql"
)

// 支持的数据源类型
const (
	DataSourceTypeCSV   = "csv"
	DataSourceTypeExcel = "excel"
	DataSourceTypeJDBC  = "jdbc"
)

// SQL 操作类型（只允许 SELECT）
const (
	SQLOpSelect = "SELECT"
)

// 默认查询限制
const (
	DefaultQueryLimit = 1000
	MaxQueryLimit     = 10000
	QueryTimeout      = 30 // 秒
)

// 语义类型
const (
	SemanticTypeID       = "id"
	SemanticTypeCurrency = "currency"
	SemanticTypeTime     = "time"
	SemanticTypeCategory = "category"
	SemanticTypeText     = "text"
	SemanticTypeNumber   = "number"
)

// 关系类型
const (
	RelationManyToOne = "many_to_one"
	RelationOneToMany = "one_to_many"
	RelationOneToOne  = "one_to_one"
)

// 数据源状态
const (
	DataSourceStatusActive   = "active"
	DataSourceStatusPending  = "pending_confirmation"
	DataSourceStatusInactive = "inactive"
)

// 执行状态
const (
	ExecutionStatusSuccess = "success"
	ExecutionStatusFailed  = "failed"
	ExecutionStatusTimeout = "timeout"
)

// 任务状态
const (
	TaskStatusPending = "pending"
	TaskStatusRunning = "running"
	TaskStatusSuccess = "success"
	TaskStatusFailed  = "failed"
)

// 任务类型
const (
	TaskTypeSchemaParam = "schema_parse"
	TaskTypeVectorIndex = "vector_index"
	TaskTypeSync        = "sync"
)

// NL2SQL 状态
const (
	NL2SQLStatusDisabled = "disabled"
	NL2SQLStatusParsing  = "parsing"
	NL2SQLStatusReady    = "ready"
	NL2SQLStatusError    = "error"
)

// 实体类型
const (
	EntityTypeDomain   = "domain"
	EntityTypeMetric   = "metric"
	EntityTypeTable    = "table"
	EntityTypeColumn   = "column"
	EntityTypeRelation = "relation"
)

// 变更类型
const (
	ChangeTypeCreate = "create"
	ChangeTypeUpdate = "update"
	ChangeTypeDelete = "delete"
)
