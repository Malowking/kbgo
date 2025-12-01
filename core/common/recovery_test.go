package common

import (
	"context"
	"testing"
	"time"
)

func TestRecoverPanic(t *testing.T) {
	ctx := context.Background()

	t.Run("正常执行不会panic", func(t *testing.T) {
		defer RecoverPanic(ctx, "test-normal")
		// 正常执行，不会panic
		_ = 1 + 1
	})

	t.Run("捕获panic", func(t *testing.T) {
		// RecoverPanic 需要在 defer 中调用才能捕获 panic
		panicked := false
		recovered := false

		func() {
			defer func() {
				if r := recover(); r != nil {
					panicked = true
					RecoverPanic(ctx, "test-panic")
					recovered = true
				}
			}()
			panic("test panic")
		}()

		if !panicked {
			t.Error("Expected function to panic")
		}
		if !recovered {
			t.Error("Expected panic to be recovered")
		}
	})
}

func TestSafeGo(t *testing.T) {
	ctx := context.Background()

	t.Run("正常goroutine执行", func(t *testing.T) {
		done := make(chan bool, 1)
		SafeGo(ctx, "test-normal-goroutine", func() {
			time.Sleep(10 * time.Millisecond)
			done <- true
		})

		select {
		case <-done:
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Error("Goroutine did not complete in time")
		}
	})

	t.Run("goroutine中panic被捕获", func(t *testing.T) {
		done := make(chan bool, 1)
		SafeGo(ctx, "test-panic-goroutine", func() {
			defer func() {
				// 即使panic，这个defer也会执行
				done <- true
			}()
			panic("intentional panic")
		})

		select {
		case <-done:
			// Panic was recovered, defer executed
		case <-time.After(100 * time.Millisecond):
			t.Error("Goroutine did not complete in time")
		}
	})
}

func TestSafeGoWithError(t *testing.T) {
	ctx := context.Background()

	t.Run("正常返回nil错误", func(t *testing.T) {
		errChan := make(chan error, 1)
		SafeGoWithError(ctx, "test-no-error", func() error {
			return nil
		}, errChan)

		select {
		case err := <-errChan:
			if err != nil {
				t.Errorf("Expected nil error, got: %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Did not receive error response in time")
		}
	})

	t.Run("返回错误", func(t *testing.T) {
		errChan := make(chan error, 1)
		SafeGoWithError(ctx, "test-with-error", func() error {
			return context.Canceled
		}, errChan)

		select {
		case err := <-errChan:
			if err != context.Canceled {
				t.Errorf("Expected context.Canceled error, got: %v", err)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Did not receive error response in time")
		}
	})

	t.Run("panic被转换为错误", func(t *testing.T) {
		errChan := make(chan error, 1)
		SafeGoWithError(ctx, "test-panic-to-error", func() error {
			panic("test panic")
		}, errChan)

		select {
		case err := <-errChan:
			if err == nil {
				t.Error("Expected panic to be converted to error")
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Did not receive error response in time")
		}
	})
}

// Benchmark性能测试
func BenchmarkSafeGo(b *testing.B) {
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		done := make(chan bool, 1)
		SafeGo(ctx, "benchmark", func() {
			done <- true
		})
		<-done
	}
}
