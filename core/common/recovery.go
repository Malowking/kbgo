package common

import (
	"context"
	"fmt"
	"runtime/debug"

	"github.com/gogf/gf/v2/frame/g"
)

// RecoverPanic 通用 panic 恢复函数
// 在 defer 中调用，捕获并记录 panic 信息（包含完整堆栈）
func RecoverPanic(ctx context.Context, taskName string) {
	if r := recover(); r != nil {
		// 获取完整堆栈信息
		stack := debug.Stack()

		// 记录详细的 panic 信息
		g.Log().Criticalf(ctx,
			"[PANIC RECOVERED] Task: %s\nError: %v\nStack Trace:\n%s",
			taskName, r, string(stack))

		// TODO: 可选 - 发送告警通知
		// 例如：发送到 Sentry, Slack, 钉钉等
		// alerting.SendPanicAlert(taskName, r, stack)
	}
}

// SafeGo 安全启动 goroutine
// 自动捕获 panic 并记录，避免 goroutine 崩溃导致程序不稳定
//
// 使用示例:
//
//	SafeGo(ctx, "process-task", func() {
//	    // 你的任务代码
//	})
func SafeGo(ctx context.Context, taskName string, fn func()) {
	go func() {
		defer RecoverPanic(ctx, taskName)
		fn()
	}()
}

// SafeGoWithError 安全启动 goroutine (带错误返回)
// 通过 channel 返回错误信息
//
// 使用示例:
//
//	errChan := make(chan error, 1)
//	SafeGoWithError(ctx, "process-task", func() error {
//	    // 你的任务代码
//	    return nil
//	}, errChan)
//	if err := <-errChan; err != nil {
//	    // 处理错误
//	}
func SafeGoWithError(ctx context.Context, taskName string, fn func() error, errChan chan<- error) {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				g.Log().Criticalf(ctx,
					"[PANIC RECOVERED] Task: %s\nError: %v\nStack Trace:\n%s",
					taskName, r, string(stack))

				errChan <- fmt.Errorf("panic in task %s: %v", taskName, r)
			}
		}()

		err := fn()
		if err != nil {
			errChan <- err
		} else {
			errChan <- nil
		}
	}()
}
