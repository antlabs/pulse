package pulse

import (
	"errors"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/antlabs/pulse/core"
	"github.com/antlabs/pulse/task/driver"
)

type Conn struct {
	fd         int64
	wbufList   []*[]byte // write buffer, 为了理精细控制内存使用量
	mu         sync.Mutex
	safeConns  *safeConns[Conn]
	task       driver.TaskExecutor
	eventLoop  core.PollingApi
	readTimer  *time.Timer
	writeTimer *time.Timer
	session    any // 会话数据
}

func (c *Conn) SetNoDelay(nodelay bool) error {
	return core.SetNoDelay(c.getFd(), nodelay)
}

func (c *Conn) getFd() int {
	return int(atomic.LoadInt64(&c.fd))
}
func newConn(fd int, safeConns *safeConns[Conn], task selectTasks, taskType TaskType, eventLoop core.PollingApi) *Conn {
	var taskExecutor driver.TaskExecutor
	switch taskType {
	case TaskTypeInConnectionGoroutine:
		taskExecutor = task.newTask("stream")
	case TaskTypeInEventLoop:
		// 不做任何事情
	case TaskTypeInBusinessGoroutine:
		taskExecutor = task.newTask("stream2")
	default:
		panic("invalid task type")
	}

	return &Conn{
		fd:        int64(fd),
		safeConns: safeConns,
		task:      taskExecutor,
		eventLoop: eventLoop,
	}
}

func (c *Conn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.close()
}

func (c *Conn) close() {
	if atomic.LoadInt64(&c.fd) == -1 {
		return
	}

	// Stop timers
	if c.readTimer != nil {
		c.readTimer.Stop()
		c.readTimer = nil
	}
	if c.writeTimer != nil {
		c.writeTimer.Stop()
		c.writeTimer = nil
	}

	oldFd := atomic.SwapInt64(&c.fd, -1)
	if oldFd != -1 {
		c.safeConns.Del(int(oldFd))
		if err := core.Close(int(oldFd)); err != nil {
			// Log the error but don't panic as this is a cleanup function
			slog.Error("failed to close fd", "fd", oldFd, "error", err)
		}
	}
	for _, wbuf := range c.wbufList {
		if wbuf != nil {
			putBytes(wbuf)
		}
	}
	c.wbufList = c.wbufList[:0]
}

// writeToSocket 尝试将数据写入 socket，并处理中断与临时错误
func (c *Conn) writeToSocket(data []byte) (int, error) {
	try := 3 //最多重试3次
	var lastErr error

	for i := 0; i < try; i++ {
		n, err := core.Write(c.getFd(), data)
		if err == nil {
			return n, nil
		}
		if err == syscall.EINTR {
			lastErr = err
			continue // 被信号中断，重试
		}
		if err == syscall.EAGAIN {
			return 0, err // 资源暂时不可用
		}
		return n, err // 其他错误直接返回
	}

	// 如果重试用尽，返回最后的错误
	return 0, lastErr
}

func (c *Conn) Write(data []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt64(&c.fd) == -1 {
		return 0, net.ErrClosed
	}

	if len(data) == 0 && len(c.wbufList) == 0 {
		return 0, nil
	}

	if len(c.wbufList) == 0 {
		n, err := c.writeToSocket(data)
		if errors.Is(err, core.EAGAIN) || errors.Is(err, core.EINTR) || err == nil {
			// 部分写入成功，或者全部失败
			// 把剩余数据放到缓冲区
			if n < 0 {
				n = 0
			}
			if n < len(data) {
				newBuf := getBytes(len(data) - n)
				copy(*newBuf, data[n:])
				if err := c.eventLoop.AddWrite(c.getFd()); err != nil {
					// 如果事件注册失败，释放缓冲区
					putBytes(newBuf)
					slog.Error("failed to add write event", "error", err)
					return n, err
				}
				c.wbufList = append(c.wbufList, newBuf)
			}
			return n, nil
		}

		// 发生严重错误
		c.close()
		return n, err
	}

	if len(data) > 0 {
		newBuf := getBytes(len(data))
		copy(*newBuf, data)
		if cap(*newBuf) < len(data) {
			panic("newBuf cap is less than data")
		}
		*newBuf = (*newBuf)[:len(data)]
		c.wbufList = append(c.wbufList, newBuf)
	}

	lastIndex := 0
	for i, wbuf := range c.wbufList {
		n, err := c.writeToSocket(*wbuf)
		if errors.Is(err, core.EAGAIN) || errors.Is(err, core.EINTR) || err == nil {
			if n < len(*wbuf) {
				// 部分写入，移动剩余数据到缓冲区开始位置
				if n > 0 {
					newBuf := getBytes(len(*wbuf) - n)
					copy(*newBuf, (*wbuf)[n:])
					putBytes(wbuf)
					c.wbufList[i] = newBuf
				}
				// 释放已处理完的缓冲区
				for j := lastIndex; j < i; j++ {
					if c.wbufList[j] == nil {
						continue
					}
					putBytes(c.wbufList[j])
					c.wbufList[j] = nil
				}
				// 移动未处理的缓冲区到列表开始位置
				copy(c.wbufList, c.wbufList[i:])
				c.wbufList = c.wbufList[:len(c.wbufList)-i]
				return len(data), nil
			}
			putBytes(wbuf)
			c.wbufList[i] = nil
			lastIndex = i + 1
			continue
		}

		// 发生严重错误
		c.close()
		return 0, err
	}

	// 所有数据都已写入
	c.wbufList = c.wbufList[:0]
	if err := c.eventLoop.ResetRead(c.getFd()); err != nil {
		slog.Error("failed to reset read event", "error", err)
	}
	return len(data), nil
}

func (c *Conn) flush() {
	if _, err := c.Write(nil); err != nil {
		slog.Error("failed to flush write buffer", "error", err)
	}
}

func (c *Conn) SetSession(session any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.session = session
}

func (c *Conn) GetSession() any {
	return c.session
}

// handleData 处理数据的逻辑
func handleData(c *Conn, options *Options, rawData []byte) {

	if options.taskType == TaskTypeInEventLoop {
		options.callback.OnData(c, rawData)
		return
	}

	var newBytes *[]byte
	newBytes = getBytes(len(rawData))
	copy(*newBytes, rawData)
	*newBytes = (*newBytes)[:len(rawData)]

	// 进入协程池
	if err := c.task.AddTask(&c.mu, func() bool {
		options.callback.OnData(c, *newBytes)
		// 释放newBytes
		if newBytes != nil {
			putBytes(newBytes)
			newBytes = nil
		}
		return true
	}); err != nil {
		slog.Error("failed to add task", "error", err)
		// 释放newBytes since task failed
		if newBytes != nil {
			putBytes(newBytes)
		}
	}
}

func (c *Conn) needFlush() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.wbufList) > 0
}

func (c *Conn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if atomic.LoadInt64(&c.fd) == -1 {
		return net.ErrClosed
	}

	// Set both read and write deadlines
	if err := c.setReadDeadlineCore(t); err != nil {
		return err
	}
	return c.setWriteDeadlineCore(t)
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.setReadDeadlineCore(t)
}

func (c *Conn) setReadDeadlineCore(t time.Time) error {

	if atomic.LoadInt64(&c.fd) == -1 {
		return net.ErrClosed
	}

	// Stop existing timer if any
	if c.readTimer != nil {
		c.readTimer.Stop()
		c.readTimer = nil
	}

	// If t is zero, we're clearing the deadline
	if t.IsZero() {
		return nil
	}

	// Create new timer
	duration := time.Until(t)
	if duration <= 0 {
		c.close()
		return nil
	}

	c.readTimer = time.AfterFunc(duration, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if atomic.LoadInt64(&c.fd) != -1 {
			c.close()
		}
	})

	return nil
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.setWriteDeadlineCore(t)
}

func (c *Conn) setWriteDeadlineCore(t time.Time) error {

	if atomic.LoadInt64(&c.fd) == -1 {
		return net.ErrClosed
	}

	// Stop existing timer if any
	if c.writeTimer != nil {
		c.writeTimer.Stop()
		c.writeTimer = nil
	}

	// If t is zero, we're clearing the deadline
	if t.IsZero() {
		return nil
	}

	// Create new timer
	duration := time.Until(t)
	if duration <= 0 {
		c.close()
		return nil
	}

	c.writeTimer = time.AfterFunc(duration, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if atomic.LoadInt64(&c.fd) != -1 {
			c.close()
		}
	})

	return nil
}
