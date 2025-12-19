package schema

import (
	"io"
	"sync"
)

// StreamReaderInterface 流式数据读取器接口
type StreamReaderInterface[T any] interface {
	Recv() (T, error)
	Close() error
}

// StreamWriterInterface 流式数据写入器接口
type StreamWriterInterface[T any] interface {
	Send(value T, err error) bool
	Close() error
}

// StreamReader 流式数据读取器（内存channel实现）
type StreamReader[T any] struct {
	ch     chan streamItem[T]
	closed bool
	mu     sync.Mutex
}

// StreamWriter 流式数据写入器（内存channel实现）
type StreamWriter[T any] struct {
	ch     chan streamItem[T]
	closed bool
	mu     sync.Mutex
}

type streamItem[T any] struct {
	value T
	err   error
}

// 确保 StreamReader 和 StreamWriter 实现接口
var _ StreamReaderInterface[any] = (*StreamReader[any])(nil)
var _ StreamWriterInterface[any] = (*StreamWriter[any])(nil)

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
// 从原始流中读取数据，并广播到所有副本流
func (r *StreamReader[T]) Copy(n int) []*StreamReader[T] {
	readers := make([]*StreamReader[T], n)
	channels := make([]chan streamItem[T], n)

	// 为每个副本创建独立的 channel
	for i := 0; i < n; i++ {
		channels[i] = make(chan streamItem[T], cap(r.ch))
		readers[i] = &StreamReader[T]{ch: channels[i]}
	}

	// 启动 goroutine 从原始流读取并广播到所有副本
	go func() {
		defer func() {
			// 关闭所有副本的 channel
			for i := 0; i < n; i++ {
				close(channels[i])
			}
		}()

		for {
			// 从原始流读取数据
			item, ok := <-r.ch
			if !ok {
				// 原始流已关闭
				r.mu.Lock()
				r.closed = true
				r.mu.Unlock()
				return
			}

			// 广播到所有副本流（每个副本只发送一次）
			// 必须确保所有副本都收到数据，所以使用阻塞发送
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
