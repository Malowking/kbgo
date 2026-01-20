package schema

// 工具错误代码常量
const (
	// ErrCodeValidation 参数验证错误
	ErrCodeValidation = "VALIDATION_ERROR"

	// ErrCodeExecution 执行错误
	ErrCodeExecution = "EXECUTION_ERROR"

	// ErrCodeTimeout 超时错误
	ErrCodeTimeout = "TIMEOUT_ERROR"

	// ErrCodeNotFound 工具未找到
	ErrCodeNotFound = "TOOL_NOT_FOUND"

	// ErrCodePermission 权限错误
	ErrCodePermission = "PERMISSION_ERROR"

	// ErrCodeNetwork 网络错误
	ErrCodeNetwork = "NETWORK_ERROR"

	// ErrCodeInternal 内部错误
	ErrCodeInternal = "INTERNAL_ERROR"
)

// NewValidationError 创建验证错误
func NewValidationError(message, details string) *ToolError {
	return &ToolError{
		Code:    ErrCodeValidation,
		Message: message,
		Details: details,
	}
}

// NewExecutionError 创建执行错误
func NewExecutionError(message, details string) *ToolError {
	return &ToolError{
		Code:    ErrCodeExecution,
		Message: message,
		Details: details,
	}
}

// NewTimeoutError 创建超时错误
func NewTimeoutError(message, details string) *ToolError {
	return &ToolError{
		Code:    ErrCodeTimeout,
		Message: message,
		Details: details,
	}
}

// NewNotFoundError 创建未找到错误
func NewNotFoundError(message, details string) *ToolError {
	return &ToolError{
		Code:    ErrCodeNotFound,
		Message: message,
		Details: details,
	}
}

// NewToolResultFromError 使用工具错误构建 ToolResult
func NewToolResultFromError(err *ToolError) *ToolResult {
	if err == nil {
		return NewToolResultWithError(
			ErrCodeInternal,
			"tool error is nil",
			"",
		)
	}
	return NewToolResultWithError(err.Code, err.Message, err.Details)
}
