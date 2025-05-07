package pulse

import (
	"errors"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/antlabs/pulse/core"
	"github.com/antlabs/pulse/task/driver"
	"golang.org/x/sys/unix"
)

type Conn struct {
	fd        int64
	wbufList  []*[]byte // write buffer, 为了理精细控制内存使用量
	mu        sync.Mutex
	safeConns *safeConns[Conn]
	task      driver.TaskExecutor
	eventLoop core.PollingApi
	closed    bool
}

func (c *Conn) getFd() int {
	return int(atomic.LoadInt64(&c.fd))
}
func newConn(fd int, safeConns *safeConns[Conn], task selectTasks, taskType TaskType, eventLoop core.PollingApi) *Conn {
	taskName := "stream2"
	if taskType == TaskTypeInConnectionGoroutine {
		taskName = "stream"
	} else if taskType == TaskTypeInEventLoop {
		taskName = "io"
	}
	return &Conn{
		fd:        int64(fd),
		safeConns: safeConns,
		task:      task.newTask(taskName),
		eventLoop: eventLoop,
	}
}

func (c *Conn) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.close()
}

func (c *Conn) close() {
	if c.closed {
		return
	}

	unix.Close(c.getFd())
	c.safeConns.Del(c.getFd())
	for _, wbuf := range c.wbufList {
		putBytes(wbuf)
	}
	c.wbufList = c.wbufList[:0]
	c.closed = true
}

// writeToSocket 尝试将数据写入 socket，并处理中断与临时错误
func (c *Conn) writeToSocket(data []byte) (int, error) {
	try := 3 //最多重试3次
	var lastErr error

	for i := 0; i < try; i++ {
		n, err := syscall.Write(c.getFd(), data)
		if err == nil {
			return n, nil
		}
		if err == syscall.EINTR {
			lastErr = err
			continue // 被信号中断，重试
		}
		if err == unix.EAGAIN {
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

	if c.closed {
		return 0, net.ErrClosed
	}

	if len(data) == 0 && len(c.wbufList) == 0 {
		return 0, nil
	}

	if len(c.wbufList) == 0 {
		n, err := c.writeToSocket(data)
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, syscall.EINTR) || err == nil {
			// 部分写入成功，或者全部失败
			if n < len(data) {
				newBuf := getBytes(len(data) - n)
				copy(*newBuf, data[n:])
				c.wbufList = append(c.wbufList, newBuf)
				c.eventLoop.AddWrite(c.getFd())
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
		if errors.Is(err, unix.EAGAIN) || errors.Is(err, syscall.EINTR) || err == nil {
			if n < len(*wbuf) {
				// 部分写入，移动剩余数据到缓冲区开始位置
				copy(*wbuf, (*wbuf)[n:])
				*wbuf = (*wbuf)[:len(*wbuf)-n]
				// 释放已处理完的缓冲区
				for j := lastIndex; j < i; j++ {
					putBytes(c.wbufList[j])
				}
				// 移动未处理的缓冲区到列表开始位置
				copy(c.wbufList, c.wbufList[i:])
				c.wbufList = c.wbufList[:len(c.wbufList)-i]
				return len(data), nil
			}
			putBytes(wbuf)
			lastIndex = i + 1
			continue
		}

		// 发生严重错误
		c.close()
		return 0, err
	}

	// 所有数据都已写入
	c.wbufList = c.wbufList[:0]
	return len(data), nil
}

func (c *Conn) flush() {
	c.Write(nil)
}

// handleData 处理数据的逻辑
func handleData[T any](c *Conn, options *Options[T], rawData []byte) {
	var data T

	var newBytes *[]byte
	// 如果配置了解码器，则尝试解码
	if options.decoder != nil {
		decodedData, _, err := options.decoder.Decode(rawData)
		if err == nil {
			data = decodedData // 使用解码后的数据
		} else {
			fmt.Println("Decode error:", err)
			return
		}
	} else {
		// 如果没有解码器，直接将原始数据转换为目标类型
		_, ok := any(rawData).(T)
		if !ok {
			fmt.Println("Type assertion failed for raw data")
			return
		}
		if options.taskType != TaskTypeInEventLoop {
			newBytes = getBytes(len(rawData))
			copy(*newBytes, rawData)
			*newBytes = (*newBytes)[:len(rawData)]
			data = any(*newBytes).(T)
		} else {
			data = any(rawData).(T)
		}
	}

	// 进入协程池
	// options.callback.OnData(c, data)
	c.task.AddTask(&c.mu, func() bool {
		options.callback.OnData(c, data)
		// 释放newBytes
		if newBytes != nil {
			putBytes(newBytes)
			newBytes = nil
		}
		return true
	})
}

func (c *Conn) needFlush() bool {
	return len(c.wbufList) > 0
}
