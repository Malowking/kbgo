package errors

// ErrCode 业务错误码类型
type ErrCode int

const (
	// 通用错误 1000-1999
	ErrInvalidParameter ErrCode = 1001 // 参数错误
	ErrUnauthorized     ErrCode = 1002 // 未授权
	ErrInternalError    ErrCode = 1003 // 内部错误
	ErrNotFound         ErrCode = 1004 // 资源未找到
	ErrAlreadyExists    ErrCode = 1005 // 资源已存在
	ErrOperationFailed  ErrCode = 1006 // 操作失败

	// 模型相关 2000-2999
	ErrModelNotFound      ErrCode = 2001 // 模型未找到
	ErrModelConfigInvalid ErrCode = 2002 // 模型配置无效
	ErrEmbeddingFailed    ErrCode = 2003 // Embedding失败
	ErrLLMCallFailed      ErrCode = 2004 // LLM调用失败
	ErrModelNotConfigured ErrCode = 2005 // 模型未配置
	ErrRerankFailed       ErrCode = 2006 // Rerank失败
	ErrStreamingFailed    ErrCode = 2007 // 流式响应失败

	// 知识库相关 3000-3999
	ErrKBNotFound      ErrCode = 3001 // 知识库未找到
	ErrKBAlreadyExists ErrCode = 3002 // 知识库已存在
	ErrKBCreateFailed  ErrCode = 3003 // 知识库创建失败
	ErrKBDeleteFailed  ErrCode = 3004 // 知识库删除失败
	ErrKBUpdateFailed  ErrCode = 3005 // 知识库更新失败

	// 文档相关 4000-4999
	ErrDocumentNotFound    ErrCode = 4001 // 文档未找到
	ErrDocumentParseFailed ErrCode = 4002 // 文档解析失败
	ErrFileAlreadyExists   ErrCode = 4003 // 文件已存在
	ErrFileSizeExceeded    ErrCode = 4004 // 文件大小超限
	ErrFileUploadFailed    ErrCode = 4005 // 文件上传失败
	ErrFileDeleteFailed    ErrCode = 4006 // 文件删除失败
	ErrFileReadFailed      ErrCode = 4007 // 文件读取失败
	ErrChunkNotFound       ErrCode = 4008 // 文档块未找到
	ErrIndexingFailed      ErrCode = 4009 // 索引失败

	// 向量数据库 5000-5999
	ErrVectorStoreInit     ErrCode = 5001 // 向量库初始化失败
	ErrVectorSearch        ErrCode = 5002 // 向量搜索失败
	ErrVectorInsert        ErrCode = 5003 // 向量插入失败
	ErrVectorDelete        ErrCode = 5004 // 向量删除失败
	ErrVectorStoreNotFound ErrCode = 5005 // 向量库不存在

	// 数据库相关 6000-6999
	ErrDatabaseQuery  ErrCode = 6001 // 数据库查询失败
	ErrDatabaseInsert ErrCode = 6002 // 数据库插入失败
	ErrDatabaseUpdate ErrCode = 6003 // 数据库更新失败
	ErrDatabaseDelete ErrCode = 6004 // 数据库删除失败
	ErrDatabaseInit   ErrCode = 6005 // 数据库初始化失败

	// 对话相关 7000-7999
	ErrConversationNotFound ErrCode = 7001 // 对话未找到
	ErrMessageNotFound      ErrCode = 7002 // 消息未找到
	ErrChatFailed           ErrCode = 7003 // 聊天失败

	// MCP相关 8000-8999
	ErrMCPServerNotFound ErrCode = 8001 // MCP服务未找到
	ErrMCPCallFailed     ErrCode = 8002 // MCP调用失败
	ErrMCPInitFailed     ErrCode = 8003 // MCP初始化失败

	// 检索相关 9000-9999
	ErrRetrievalFailed ErrCode = 9001 // 检索失败
	ErrRewriteFailed   ErrCode = 9002 // 查询重写失败
)

// HTTPStatusCode 返回错误码对应的HTTP状态码
func (e ErrCode) HTTPStatusCode() int {
	switch {
	case e >= 1001 && e <= 1999:
		// 通用错误
		switch e {
		case ErrInvalidParameter:
			return 400
		case ErrUnauthorized:
			return 401
		case ErrNotFound:
			return 404
		case ErrAlreadyExists:
			return 409
		default:
			return 500
		}
	case e >= 2000 && e <= 2999:
		// 模型相关错误
		if e == ErrModelNotFound {
			return 404
		}
		return 500
	case e >= 3000 && e <= 3999:
		// 知识库相关错误
		switch e {
		case ErrKBNotFound:
			return 404
		case ErrKBAlreadyExists:
			return 409
		default:
			return 500
		}
	case e >= 4000 && e <= 4999:
		// 文档相关错误
		switch e {
		case ErrDocumentNotFound, ErrChunkNotFound:
			return 404
		case ErrFileAlreadyExists:
			return 409
		case ErrFileSizeExceeded:
			return 413
		default:
			return 500
		}
	default:
		return 500
	}
}
