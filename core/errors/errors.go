package errors

import (
	"fmt"
)

// AppError 应用业务错误
type AppError struct {
	Code    ErrCode // 业务错误码
	Message string  // 错误消息
}

// Error 实现 error 接口
func (e *AppError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// New 创建新的业务错误
func New(code ErrCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

// Newf 创建新的业务错误（格式化消息）
func Newf(code ErrCode, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
	}
}

// IsAppError 判断是否为业务错误
func IsAppError(err error) bool {
	_, ok := err.(*AppError)
	return ok
}

// GetAppError 获取业务错误，如果不是则返回nil
func GetAppError(err error) *AppError {
	if appErr, ok := err.(*AppError); ok {
		return appErr
	}
	return nil
}
