package v1

import (
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

// ============ NL2SQL 数据源管理接口 ============

// NL2SQLCreateDataSourceReq 创建数据源请求
type NL2SQLCreateDataSourceReq struct {
	g.Meta           `path:"/v1/nl2sql/datasources" method:"post" tags:"nl2sql" summary:"创建NL2SQL数据源"`
	Name             string                 `json:"name" v:"required#数据源名称不能为空"`                       // 数据源名称
	Type             string                 `json:"type" v:"required#类型不能为空"`                          // 类型: jdbc, csv, excel
	DBType           string                 `json:"db_type"`                                           // 数据库类型: postgresql
	Config           map[string]interface{} `json:"config"`                                            // 配置信息
	CreatedBy        string                 `json:"created_by" v:"required#创建人不能为空"`                   // 创建人
	EmbeddingModelID string                 `json:"embedding_model_id" v:"required#Embedding模型ID不能为空"` // Embedding模型ID（用于创建向量表）
}

// NL2SQLCreateDataSourceRes 创建数据源响应
type NL2SQLCreateDataSourceRes struct {
	DatasourceID     string `json:"datasource_id"`      // 数据源ID
	Status           string `json:"status"`             // 状态
	CollectionName   string `json:"collection_name"`    // 向量表名称
	VectorStoreReady bool   `json:"vector_store_ready"` // 向量存储是否就绪
}

// NL2SQLUploadFileReq 上传CSV/Excel数据文件请求
type NL2SQLUploadFileReq struct {
	g.Meta      `path:"/v1/nl2sql/upload-file" method:"post" mime:"multipart/form-data" tags:"nl2sql" summary:"上传CSV/Excel数据文件"`
	File        *ghttp.UploadFile `p:"file" type:"file" dc:"Upload CSV or Excel file" v:"required"`
	Name        string            `p:"name" dc:"Data source name" v:"required"`
	DisplayName string            `p:"display_name" dc:"Table display name (optional, defaults to filename)"` // 表显示名称（可选）
	CreatedBy   string            `p:"created_by" dc:"Creator user ID" v:"required"`
}

// NL2SQLUploadFileRes 上传数据文件响应
type NL2SQLUploadFileRes struct {
	DatasourceID string `json:"datasource_id"` // 数据源ID
	FilePath     string `json:"file_path"`     // 文件路径
	Status       string `json:"status"`        // 状态
	Message      string `json:"message"`       // 消息
}

// NL2SQLParseSchemaReq 解析Schema请求
type NL2SQLParseSchemaReq struct {
	g.Meta           `path:"/v1/nl2sql/datasources/:datasource_id/parse" method:"post" tags:"nl2sql" summary:"解析数据源Schema"`
	DatasourceID     string `json:"datasource_id" v:"required#数据源ID不能为空"`  // 数据源ID
	LLMModelID       string `json:"llm_model_id" v:"required#LLM模型ID不能为空"` // LLM模型ID（用于增强描述）
	EmbeddingModelID string `json:"embedding_model_id"`                    // Embedding模型ID（可选，不传则使用数据源绑定的模型）
}

// NL2SQLParseSchemaRes 解析Schema响应
type NL2SQLParseSchemaRes struct {
	TaskID string `json:"task_id"` // 异步任务ID
}

// NL2SQLGetTaskReq 获取任务状态请求
type NL2SQLGetTaskReq struct {
	g.Meta `path:"/v1/nl2sql/tasks/:task_id" method:"get" tags:"nl2sql" summary:"获取Schema解析任务状态"`
	TaskID string `json:"task_id" v:"required#任务ID不能为空"` // 任务ID
}

// NL2SQLGetTaskRes 获取任务状态响应
type NL2SQLGetTaskRes struct {
	TaskID      string                 `json:"task_id"`      // 任务ID
	Status      string                 `json:"status"`       // 状态: pending, running, success, failed
	Progress    int                    `json:"progress"`     // 进度 0-100
	CurrentStep string                 `json:"current_step"` // 当前步骤
	ErrorMsg    string                 `json:"error_msg"`    // 错误信息
	Result      map[string]interface{} `json:"result"`       // 结果（成功时）
}

// NL2SQLListDataSourcesReq 列出数据源请求
type NL2SQLListDataSourcesReq struct {
	g.Meta  `path:"/v1/nl2sql/datasources" method:"get" tags:"nl2sql" summary:"获取数据源列表"`
	AgentID string `json:"agent_id"` // Agent预设ID
	Status  string `json:"status"`   // 状态
	Page    int    `json:"page" v:"min:1#页码必须大于0" d:"1"`
	Size    int    `json:"size" v:"min:1|max:100#每页数量必须在1-100之间" d:"10"`
}

// NL2SQLListDataSourcesRes 列出数据源响应
type NL2SQLListDataSourcesRes struct {
	List  []*NL2SQLDataSourceItem `json:"list"`  // 数据源列表
	Total int64                   `json:"total"` // 总数
	Page  int                     `json:"page"`  // 当前页
	Size  int                     `json:"size"`  // 每页数量
}

// NL2SQLDataSourceItem 数据源列表项
type NL2SQLDataSourceItem struct {
	ID               string `json:"id"`                 // 数据源ID
	Name             string `json:"name"`               // 名称
	Type             string `json:"type"`               // 类型
	DBType           string `json:"db_type"`            // 数据库类型
	Status           string `json:"status"`             // 状态
	EmbeddingModelID string `json:"embedding_model_id"` // Embedding模型ID
	CreatedAt        string `json:"create_time"`        // 创建时间 (前端期望的字段名)
	UpdatedAt        string `json:"update_time"`        // 更新时间 (前端期望的字段名)
}

// NL2SQLDeleteDataSourceReq 删除数据源请求
type NL2SQLDeleteDataSourceReq struct {
	g.Meta       `path:"/v1/nl2sql/datasources/:datasource_id" method:"delete" tags:"nl2sql" summary:"删除数据源"`
	DatasourceID string `json:"datasource_id" v:"required#数据源ID不能为空"` // 数据源ID
}

// NL2SQLDeleteDataSourceRes 删除数据源响应
type NL2SQLDeleteDataSourceRes struct {
	Success bool `json:"success"` // 是否成功
}

// ============ NL2SQL 查询接口 ============

// NL2SQLQueryReq NL2SQL查询请求
type NL2SQLQueryReq struct {
	g.Meta       `path:"/v1/nl2sql/query" method:"post" tags:"nl2sql" summary:"执行NL2SQL查询"`
	DatasourceID string `json:"datasource_id" v:"required#数据源ID不能为空"`  // 数据源ID
	Question     string `json:"question" v:"required#问题不能为空"`          // 自然语言问题
	SessionID    string `json:"session_id"`                            // 会话ID（可选，用于上下文）
	LLMModelID   string `json:"llm_model_id" v:"required#LLM模型ID不能为空"` // LLM模型ID（用于生成SQL）
}

// NL2SQLQueryRes NL2SQL查询响应
type NL2SQLQueryRes struct {
	QueryLogID  string                   `json:"query_log_id"`         // 查询日志ID
	SQL         string                   `json:"sql"`                  // 生成的SQL
	Result      *NL2SQLQueryResult       `json:"result,omitempty"`     // 查询结果
	Explanation string                   `json:"explanation"`          // SQL解释
	Error       string                   `json:"error,omitempty"`      // 错误信息
	References  []*NL2SQLSchemaReference `json:"references,omitempty"` // 引用的Schema
}

// NL2SQLQueryResult 查询结果
type NL2SQLQueryResult struct {
	Columns  []string                 `json:"columns"`   // 列名
	Data     []map[string]interface{} `json:"data"`      // 数据行
	RowCount int                      `json:"row_count"` // 行数
}

// NL2SQLSchemaReference Schema引用
type NL2SQLSchemaReference struct {
	Type        string  `json:"type"`        // 类型: table, column, metric
	Name        string  `json:"name"`        // 名称
	Description string  `json:"description"` // 描述
	Score       float64 `json:"score"`       // 相似度分数
}

// NL2SQLFeedbackReq 用户反馈请求
type NL2SQLFeedbackReq struct {
	g.Meta     `path:"/v1/nl2sql/feedback" method:"post" tags:"nl2sql" summary:"提交NL2SQL查询反馈"`
	QueryLogID string `json:"query_log_id" v:"required#查询日志ID不能为空"` // 查询日志ID
	Feedback   string `json:"feedback" v:"required#反馈不能为空"`         // 反馈: positive, negative
	Comment    string `json:"comment"`                              // 反馈备注
}

// NL2SQLFeedbackRes 用户反馈响应
type NL2SQLFeedbackRes struct {
	Success bool `json:"success"` // 是否成功
}

// ============ NL2SQL Schema查看接口 ============

// NL2SQLGetSchemaReq 获取Schema请求
type NL2SQLGetSchemaReq struct {
	g.Meta       `path:"/v1/nl2sql/datasources/:datasource_id/schema" method:"get" tags:"nl2sql" summary:"获取数据源Schema"`
	DatasourceID string `json:"datasource_id" v:"required#数据源ID不能为空"` // 数据源ID
}

// NL2SQLGetSchemaRes 获取Schema响应
type NL2SQLGetSchemaRes struct {
	SchemaID  string               `json:"schema_id"` // Schema ID
	Tables    []*NL2SQLTableDetail `json:"tables"`    // 表列表（详细信息）
	Relations []*NL2SQLRelation    `json:"relations"` // 表关系
	Metrics   []*NL2SQLMetric      `json:"metrics"`   // 指标列表
	Domains   []*NL2SQLDomain      `json:"domains"`   // 业务域列表
}

// NL2SQLTableDetail 表详细信息（用于Schema展示）
type NL2SQLTableDetail struct {
	ID          string                `json:"id"`           // 表ID
	TableName   string                `json:"table_name"`   // 表名
	DisplayName string                `json:"display_name"` // 显示名称
	Description string                `json:"description"`  // 描述
	RowCount    int64                 `json:"row_count"`    // 行数
	Parsed      bool                  `json:"parsed"`       // 是否已解析
	Columns     []*NL2SQLColumnDetail `json:"columns"`      // 列列表
	PrimaryKeys []string              `json:"primary_keys"` // 主键列表
}

// NL2SQLColumnDetail 列详细信息（用于Schema展示）
type NL2SQLColumnDetail struct {
	ID          string `json:"id"`          // 列ID
	ColumnName  string `json:"column_name"` // 列名
	DataType    string `json:"data_type"`   // 数据类型
	Description string `json:"description"` // 描述
	Nullable    bool   `json:"nullable"`    // 是否可空
}

// NL2SQLRelation 表关系
type NL2SQLRelation struct {
	ID           string `json:"id"`            // 关系ID
	SourceTable  string `json:"source_table"`  // 源表
	TargetTable  string `json:"target_table"`  // 目标表
	RelationType string `json:"relation_type"` // 关系类型
}

// NL2SQLTable 表信息
type NL2SQLTable struct {
	TableName   string          `json:"table_name"`  // 表名
	Description string          `json:"description"` // 描述
	RowCount    int64           `json:"row_count"`   // 行数
	Columns     []*NL2SQLColumn `json:"columns"`     // 列列表
}

// NL2SQLColumn 列信息
type NL2SQLColumn struct {
	ColumnName   string `json:"column_name"`    // 列名
	DataType     string `json:"data_type"`      // 数据类型
	Description  string `json:"description"`    // 描述
	IsPrimaryKey bool   `json:"is_primary_key"` // 是否主键
	IsNullable   bool   `json:"is_nullable"`    // 是否可空
	SampleValues string `json:"sample_values"`  // 示例值
}

// NL2SQLMetric 指标信息
type NL2SQLMetric struct {
	MetricName  string `json:"metric_name"` // 指标名称
	Description string `json:"description"` // 描述
	Formula     string `json:"formula"`     // 计算公式
}

// NL2SQLDomain 业务域信息
type NL2SQLDomain struct {
	DomainName  string `json:"domain_name"` // 业务域名称
	Description string `json:"description"` // 描述
	TableNames  string `json:"table_names"` // 关联的表名
}

// ============ NL2SQL 添加表到数据源接口 ============

// NL2SQLAddTableReq 添加表到数据源请求
type NL2SQLAddTableReq struct {
	g.Meta       `path:"/v1/nl2sql/datasources/:datasource_id/tables" method:"post" mime:"multipart/form-data" tags:"nl2sql" summary:"添加表到现有数据源"`
	DatasourceID string            `p:"datasource_id" v:"required#数据源ID不能为空"` // 数据源ID
	File         *ghttp.UploadFile `p:"file" type:"file" dc:"Upload CSV or Excel file" v:"required"`
	DisplayName  string            `p:"display_name" dc:"Table display name (optional, defaults to filename)"` // 表显示名称（可选）
}

// NL2SQLAddTableRes 添加表到数据源响应
type NL2SQLAddTableRes struct {
	DatasourceID string `json:"datasource_id"` // 数据源ID
	TableName    string `json:"table_name"`    // 表名
	RowCount     int    `json:"row_count"`     // 行数
	Status       string `json:"status"`        // 状态
	Message      string `json:"message"`       // 消息
}

// ============ NL2SQL Metrics 管理接口 ============

// NL2SQLCreateMetricReq 创建预定义指标请求
type NL2SQLCreateMetricReq struct {
	g.Meta         `path:"/v1/nl2sql/metrics" method:"post" tags:"nl2sql" summary:"创建预定义指标"`
	DatasourceID   string                 `json:"datasource_id" v:"required#数据源ID不能为空"`
	MetricID       string                 `json:"metric_id" v:"required#指标ID不能为空"`
	Name           string                 `json:"name" v:"required#指标名称不能为空"`
	Description    string                 `json:"description"`
	Formula        string                 `json:"formula" v:"required#计算公式不能为空"`
	DefaultFilters map[string]interface{} `json:"default_filters"`
	TimeColumn     string                 `json:"time_column"`
}

// NL2SQLCreateMetricRes 创建预定义指标响应
type NL2SQLCreateMetricRes struct {
	MetricID string `json:"metric_id"`
	Message  string `json:"message"`
}

// NL2SQLListMetricsReq 查询指标列表请求
type NL2SQLListMetricsReq struct {
	g.Meta       `path:"/v1/nl2sql/metrics" method:"get" tags:"nl2sql" summary:"查询指标列表"`
	DatasourceID string `json:"datasource_id" v:"required#数据源ID不能为空"`
}

// NL2SQLListMetricsRes 查询指标列表响应
type NL2SQLListMetricsRes struct {
	Metrics []NL2SQLMetricInfo `json:"metrics"`
}

// NL2SQLMetricInfo 指标详细信息
type NL2SQLMetricInfo struct {
	ID             string                 `json:"id"`
	MetricID       string                 `json:"metric_id"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Formula        string                 `json:"formula"`
	DefaultFilters map[string]interface{} `json:"default_filters"`
	TimeColumn     string                 `json:"time_column"`
	CreateTime     string                 `json:"create_time"`
}

// NL2SQLDeleteMetricReq 删除指标请求
type NL2SQLDeleteMetricReq struct {
	g.Meta   `path:"/v1/nl2sql/metrics/:metric_id" method:"delete" tags:"nl2sql" summary:"删除指标"`
	MetricID string `json:"metric_id" v:"required#指标ID不能为空"`
}

// NL2SQLDeleteMetricRes 删除指标响应
type NL2SQLDeleteMetricRes struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ============ NL2SQL Relations 管理接口 ============

// NL2SQLCreateRelationReq 手动创建表关系请求
type NL2SQLCreateRelationReq struct {
	g.Meta       `path:"/v1/nl2sql/relations" method:"post" tags:"nl2sql" summary:"创建表关系"`
	DatasourceID string `json:"datasource_id" v:"required#数据源ID不能为空"`
	FromTableID  string `json:"from_table_id" v:"required#源表ID不能为空"`
	FromColumn   string `json:"from_column" v:"required#源列名不能为空"`
	ToTableID    string `json:"to_table_id" v:"required#目标表ID不能为空"`
	ToColumn     string `json:"to_column" v:"required#目标列名不能为空"`
	RelationType string `json:"relation_type" v:"required|in:many_to_one,one_to_many,one_to_one#关系类型不合法"`
	JoinType     string `json:"join_type" v:"in:INNER,LEFT,RIGHT#JOIN类型不合法" d:"INNER"`
	Description  string `json:"description"`
}

// NL2SQLCreateRelationRes 创建表关系响应
type NL2SQLCreateRelationRes struct {
	RelationID string `json:"relation_id"`
	Message    string `json:"message"`
}

// NL2SQLListRelationsReq 查询表关系列表请求
type NL2SQLListRelationsReq struct {
	g.Meta       `path:"/v1/nl2sql/relations" method:"get" tags:"nl2sql" summary:"查询表关系列表"`
	DatasourceID string `json:"datasource_id" v:"required#数据源ID不能为空"`
}

// NL2SQLListRelationsRes 查询表关系列表响应
type NL2SQLListRelationsRes struct {
	Relations []NL2SQLRelationInfo `json:"relations"`
}

// NL2SQLRelationInfo 表关系详细信息
type NL2SQLRelationInfo struct {
	ID            string `json:"id"`
	RelationID    string `json:"relation_id"`
	FromTableName string `json:"from_table_name"`
	FromColumn    string `json:"from_column"`
	ToTableName   string `json:"to_table_name"`
	ToColumn      string `json:"to_column"`
	RelationType  string `json:"relation_type"`
	JoinType      string `json:"join_type"`
	Description   string `json:"description"`
	CreateTime    string `json:"create_time"`
}

// NL2SQLDeleteRelationReq 删除表关系请求
type NL2SQLDeleteRelationReq struct {
	g.Meta     `path:"/v1/nl2sql/relations/:relation_id" method:"delete" tags:"nl2sql" summary:"删除表关系"`
	RelationID string `json:"relation_id" v:"required#关系ID不能为空"`
}

// NL2SQLDeleteRelationRes 删除表关系响应
type NL2SQLDeleteRelationRes struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// NL2SQLDeleteTableReq 删除表请求
type NL2SQLDeleteTableReq struct {
	g.Meta       `path:"/v1/nl2sql/datasources/:datasource_id/tables/:table_id" method:"delete" tags:"nl2sql" summary:"删除数据源中的表"`
	DatasourceID string `json:"datasource_id" v:"required#数据源ID不能为空"` // 数据源ID
	TableID      string `json:"table_id" v:"required#表ID不能为空"`        // 表ID
}

// NL2SQLDeleteTableRes 删除表响应
type NL2SQLDeleteTableRes struct {
	Success bool   `json:"success"` // 是否成功
	Message string `json:"message"` // 消息
}
