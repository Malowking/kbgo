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

		// 发送告警通知（可选）
		// 在生产环境中应该配置告警服务，例如：
		// 1. 集成 Sentry: sentry.CaptureException(fmt.Errorf("%v", r))
		// 2. 发送到钉钉/企业微信: SendDingTalkAlert(taskName, r, stack)
		// 3. 发送到 Slack: SendSlackAlert(taskName, r, stack)
		//
		// 示例配置（需要在配置文件中启用）:
		// if alertConfig := g.Cfg().Get(ctx, "alert"); alertConfig != nil {
		//     SendPanicAlert(ctx, taskName, r, stack)
		// }
		g.Log().Noticef(ctx, "[ALERT] 生产环境应配置告警通知服务，当前仅记录日志")
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
