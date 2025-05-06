package pulse

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"syscall"

	"github.com/antlabs/pulse/core"
	"github.com/antlabs/pulse/task/driver"
	"golang.org/x/sys/unix"
)

type Conn struct {
	fd        int64
	wbuf      *[]byte // write buffer, 为了理精细控制内存使用量
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
	unix.Close(c.getFd())
	c.safeConns.Del(c.getFd())
	if c.wbuf != nil {
		putBytes(c.wbuf)
		c.wbuf = nil
	}
}

func (c *Conn) Write(data []byte) (int, error) {
	dataLen := len(data)
	if dataLen == 0 {
		return 0, nil
	}

	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return 0, errors.New("connection closed")
	}
	if c.wbuf == nil {
		n, err := syscall.Write(c.getFd(), data)
		if err == unix.EAGAIN || err == syscall.EINTR {
			// 资源暂时不可用或被信号中断，需要缓存数据
			newBytes := getBytes(len(data))
			*newBytes = (*newBytes)[:len(data)]
			c.wbuf = newBytes
			if n > 0 {
				// 部分写成功
				copy(*c.wbuf, data[n:])
				*c.wbuf = (*c.wbuf)[:len(data)-n]
			} else {
				// 全部写失败
				copy(*c.wbuf, data)
				*c.wbuf = (*c.wbuf)[:len(data)]
			}

			// 注册写事件，当socket可写时会通知我们
			c.eventLoop.AddWrite(c.getFd())
			c.mu.Unlock()
			return n, nil
		}
		c.mu.Unlock()
		return n, err
	}

	// 将本次数据追加到写缓冲区
	*c.wbuf = append(*c.wbuf, data...)

	// 尝试写入缓冲区的所有数据
	n, err := syscall.Write(c.getFd(), *c.wbuf)
	if err != nil {
		if err == unix.EAGAIN || err == syscall.EINTR {
			// 如果是临时错误，确保我们仍然注册了写事件
			c.eventLoop.AddWrite(c.getFd())
			c.mu.Unlock()
			return 0, nil
		}

		// 处理其他错误情况，确保不会泄漏内存
		c.close()
		c.mu.Unlock()
		return 0, err
	}

	if n == len(*c.wbuf) {
		// 全部写入成功
		putBytes(c.wbuf)
		c.wbuf = nil
	} else if n > 0 {
		// 部分写入成功，更新缓冲区
		copy(*c.wbuf, (*c.wbuf)[n:])
		*c.wbuf = (*c.wbuf)[:len(*c.wbuf)-n]

		// c.eventLoop.AddWrite(c.getFd())
	}

	c.mu.Unlock()

	return n, nil
}

func (c *Conn) flush() {
	c.mu.Lock()
	if c.wbuf == nil || len(*c.wbuf) == 0 {
		c.mu.Unlock()
		return
	}

	if c.closed {
		c.mu.Unlock()
		return
	}

	fd := c.getFd()
	n, err := syscall.Write(fd, *c.wbuf)
	if err != nil {
		if err == unix.EAGAIN || err == syscall.EINTR {
			// 临时错误，确保我们仍然注册了写事件
			c.eventLoop.AddWrite(fd)
			c.mu.Unlock()
			return
		}

		// 处理非临时错误：清理资源并关闭连接
		c.close()
		c.mu.Unlock()
		return
	}

	if n == len(*c.wbuf) {
		// 全部写入成功
		putBytes(c.wbuf)
		c.wbuf = nil
		// 不再需要写事件，只监听读事件
		c.eventLoop.ResetRead(fd)
	} else if n > 0 {
		// 部分写入成功，更新缓冲区
		copy(*c.wbuf, (*c.wbuf)[n:])
		*c.wbuf = (*c.wbuf)[:len(*c.wbuf)-n]
	}
	c.mu.Unlock()
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
	return c.wbuf != nil && len(*c.wbuf) > 0
}
