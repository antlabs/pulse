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
	"github.com/antlabs/task/task/driver"
)

type Conn struct {
	fd         int64
	wbufList   []*[]byte // write buffer, 为了理精细控制内存使用量
	mu         sync.Mutex
	safeConns  *core.SafeConns[Conn]
	task       driver.TaskExecutor
	eventLoop  core.PollingApi
	readTimer  *time.Timer
	writeTimer *time.Timer
	session    any // 会话数据

	// 如果再加字段，可以改成对options的指针的访问, 目前只是浪费了8个字节
	readBufferSize             int  // 读缓冲区大小
	flowBackPressureRemoveRead bool // 流量背压机制，当连接的写缓冲区满了，会移除读事件
	readableButNotRead         bool // 垂直触发模式下,表示可读未读取的标记位
}

func (c *Conn) SetNoDelay(nodelay bool) error {
	return core.SetNoDelay(c.getFd(), nodelay)
}

func (c *Conn) getFd() int {
	return int(atomic.LoadInt64(&c.fd))
}

func newConn(fd int, safeConns *core.SafeConns[Conn],
	task selectTasks, taskType TaskType,
	eventLoop core.PollingApi, readBufferSize int, flowBackPressureRemoveRead bool) *Conn {
	var taskExecutor driver.TaskExecutor
	switch taskType {
	case TaskTypeInConnectionGoroutine:
		taskExecutor = task.newTask("onebyone")
	case TaskTypeInEventLoop:
		// 不做任何事情
	case TaskTypeInBusinessGoroutine:
		taskExecutor = task.newTask("stream")
	default:
		panic("invalid task type")
	}

	return &Conn{
		fd:                         int64(fd),
		safeConns:                  safeConns,
		task:                       taskExecutor,
		eventLoop:                  eventLoop,
		readBufferSize:             readBufferSize,
		flowBackPressureRemoveRead: flowBackPressureRemoveRead,
	}
}

func (c *Conn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closeNoLock()
}

func (c *Conn) closeNoLock() {
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

	n, err := core.Write(c.getFd(), data)
	if err == nil {
		return n, nil
	}
	if err == syscall.EINTR {
		return 0, err // 被信号中断，直接返回
	}
	if err == syscall.EAGAIN {
		return 0, err // 资源暂时不可用
	}
	return 0, err // 其他错误直接返回

}

// appendToWbufList 将数据添加到写缓冲区列表
// 先检查最后一个缓冲区是否有足够空间，如果有就直接append
// 如果没有，将部分数据append到最后一个缓冲区，剩余部分创建新的readBufferSize大小的缓冲区
func (c *Conn) appendToWbufList(data []byte) {
	if len(data) == 0 {
		return
	}

	// 如果wbufList为空，直接创建新的缓冲区
	if len(c.wbufList) == 0 {
		newBuf := getBytesWithSize(len(data), c.readBufferSize)
		copy(*newBuf, data)
		*newBuf = (*newBuf)[:len(data)]
		c.wbufList = append(c.wbufList, newBuf)
		return
	}

	// 获取最后一个缓冲区
	lastBuf := c.wbufList[len(c.wbufList)-1]
	remainingSpace := cap(*lastBuf) - len(*lastBuf)

	// 如果最后一个缓冲区有足够空间，直接append
	if remainingSpace >= len(data) {
		*lastBuf = append(*lastBuf, data...)
		return
	}

	// 如果空间不够，先填满最后一个缓冲区
	if remainingSpace > 0 {
		*lastBuf = append(*lastBuf, data[:remainingSpace]...)
		data = data[remainingSpace:] // 剩余的数据
	}

	// 为剩余数据创建新的缓冲区（使用readBufferSize大小）
	for len(data) > 0 {
		newBuf := getBytesWithSize(len(data), c.readBufferSize)
		copySize := len(data)
		if copySize > cap(*newBuf) {
			copySize = cap(*newBuf)
		}
		copy(*newBuf, data[:copySize])
		*newBuf = (*newBuf)[:copySize]
		c.wbufList = append(c.wbufList, newBuf)
		data = data[copySize:]
	}
}

// handlePartialWrite 处理部分写入的情况，创建新缓冲区存储剩余数据
func (c *Conn) handlePartialWrite(data *[]byte, n int, needAppend bool) error {
	if n < 0 {
		n = 0
	}

	// 如果已经全部写入，不需要创建新缓冲区
	if n >= len(*data) {
		return nil
	}

	remainingData := (*data)[n:]
	if needAppend {
		c.appendToWbufList(remainingData)
	} else {
		copy(*data, (*data)[n:])
		*data = (*data)[:len(*data)-n]
	}

	// 部分写入成功，或者全部失败
	// 如果启用了流量背压机制且有部分写入，先删除读事件
	if c.flowBackPressureRemoveRead {
		if delErr := c.eventLoop.DelRead(c.getFd()); delErr != nil {
			slog.Error("failed to delete read event", "error", delErr)
		}
	} else {
		if err := c.eventLoop.AddWrite(c.getFd()); err != nil {
			slog.Error("failed to add write event", "error", err)
			return err
		}
	}

	return nil
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
			if n == len(data) {
				return n, nil
			}
			// 把剩余数据放到缓冲区
			if err := c.handlePartialWrite(&data, n, true); err != nil {
				c.closeNoLock()
				return 0, err
			}
			return len(data), nil
		}

		// 发生严重错误
		c.closeNoLock()
		return n, err
	}

	if len(data) > 0 {
		c.appendToWbufList(data)
	}

	i := 0
	for i < len(c.wbufList) {
		wbuf := c.wbufList[i]
		n, err := c.writeToSocket(*wbuf)
		if errors.Is(err, core.EAGAIN) || errors.Is(err, core.EINTR) || err == nil /*写入成功，也有n != len(*wbuf)的情况*/ {
			if n == len(*wbuf) {
				putBytes(wbuf)
				c.wbufList[i] = nil
				i++
				continue
			}
			// 移动剩余数据到缓冲区开始位置
			if err := c.handlePartialWrite(wbuf, n, false); err != nil {
				c.closeNoLock()
				return 0, err
			}

			// 移动未处理的缓冲区到列表开始位置
			copy(c.wbufList, c.wbufList[i:])
			c.wbufList = c.wbufList[:len(c.wbufList)-i]
			return len(data), nil
		}

		c.closeNoLock()
		return n, err
	}

	// 所有数据都已写入
	c.wbufList = c.wbufList[:0]
	// 需要进的逻辑
	// 1.如果是垂直触发模式，并且启用了流量背压机制，重新添加读事件
	// 2.如果是水平触发模式也重新添加读事件，为了去掉写事件
	// 3.如果是垂直触发模式，并且启用了流量背压机制，则需要添加读事件

	// 不需要进的逻辑
	// 1.如果是垂直触发模式，并且没有启用流量背压机制，不需要重新添加事件, TODO

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
	newBytes = getBytesWithSize(len(rawData), c.readBufferSize)
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
		c.closeNoLock()
		return nil
	}

	c.readTimer = time.AfterFunc(duration, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if atomic.LoadInt64(&c.fd) != -1 {
			c.closeNoLock()
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
		c.closeNoLock()
		return nil
	}

	c.writeTimer = time.AfterFunc(duration, func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		if atomic.LoadInt64(&c.fd) != -1 {
			c.closeNoLock()
		}
	})

	return nil
}
