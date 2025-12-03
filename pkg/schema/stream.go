package schema

import (
	"io"
	"sync"
)

// StreamReader 流式数据读取器
type StreamReader[T any] struct {
	ch     chan streamItem[T]
	closed bool
	mu     sync.Mutex
}

// StreamWriter 流式数据写入器
type StreamWriter[T any] struct {
	ch     chan streamItem[T]
	closed bool
	mu     sync.Mutex
}

type streamItem[T any] struct {
	value T
	err   error
}

// Pipe 创建一个流式管道，返回 Reader 和 Writer
func Pipe[T any](bufferSize int) (*StreamReader[T], *StreamWriter[T]) {
	ch := make(chan streamItem[T], bufferSize)
	return &StreamReader[T]{ch: ch}, &StreamWriter[T]{ch: ch}
}

// Recv 从流中读取下一个元素
// 返回值和错误。当流结束时返回 io.EOF
func (r *StreamReader[T]) Recv() (T, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		var zero T
		return zero, io.EOF
	}
	r.mu.Unlock()

	item, ok := <-r.ch
	if !ok {
		r.mu.Lock()
		r.closed = true
		r.mu.Unlock()
		var zero T
		return zero, io.EOF
	}

	return item.value, item.err
}

// Close 关闭读取器
func (r *StreamReader[T]) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.closed {
		r.closed = true
	}
	return nil
}

// Copy creates n copies of the StreamReader
func (r *StreamReader[T]) Copy(n int) []*StreamReader[T] {
	readers := make([]*StreamReader[T], n)
	channels := make([]chan streamItem[T], n)

	for i := 0; i < n; i++ {
		channels[i] = make(chan streamItem[T], cap(r.ch))
		readers[i] = &StreamReader[T]{ch: channels[i]}
	}

	go func() {
		defer func() {
			for i := 0; i < n; i++ {
				close(channels[i])
			}
		}()

		for {
			r.mu.Lock()
			if r.closed {
				r.mu.Unlock()
				return
			}
			r.mu.Unlock()

			item, ok := <-r.ch
			if !ok {
				r.mu.Lock()
				r.closed = true
				r.mu.Unlock()
				return
			}

			// Send to all channels
			for i := 0; i < n; i++ {
				select {
				case channels[i] <- item:
				default:
					// If channel is full, we still need to send to remaining channels
					// This maintains consistency across all copies
				}
			}

			// Ensure all channels receive the item
			for i := 0; i < n; i++ {
				channels[i] <- item
			}
		}
	}()

	return readers
}

// Send 向流中发送一个元素
// 返回 true 表示流已关闭
func (w *StreamWriter[T]) Send(value T, err error) bool {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return true
	}
	w.mu.Unlock()

	select {
	case w.ch <- streamItem[T]{value: value, err: err}:
		return false
	default:
		// Channel已满或已关闭
		return true
	}
}

// Close 关闭写入器
func (w *StreamWriter[T]) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.closed {
		w.closed = true
		close(w.ch)
	}
	return nil
}
