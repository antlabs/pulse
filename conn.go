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

func (c *Conn) Write(data []byte) (int, error) {
	dataLen := len(data)
	if dataLen == 0 {
		return 0, nil
	}

	c.mu.Lock()
	if c.wbuf == nil {
		n, err := syscall.Write(c.getFd(), data)
		if err == unix.EAGAIN {
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

			c.eventLoop.AddWrite(c.getFd())
			c.mu.Unlock()
			return n, nil
		}
		c.mu.Unlock()
		return n, err
	}

	// 缓冲区已经存在，说明之前的写入还未完成
	// 将本次数据追加到写缓冲区
	originalBufferLen := len(*c.wbuf)
	*c.wbuf = append(*c.wbuf, data...)

	// 尝试写入缓冲区的所有数据
	n, err := syscall.Write(c.getFd(), *c.wbuf)
	if err != nil && err != unix.EAGAIN {
		// 处理错误情况，确保不会泄漏内存
		putBytes(c.wbuf)
		c.wbuf = nil
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
	}

	c.mu.Unlock()

	// 计算有多少当前请求的数据被写入
	if n <= originalBufferLen {
		// 如果写入量小于等于原缓冲区大小，说明当前数据都未写入
		return 0, nil
	} else {
		// 部分或全部被写入
		return n - originalBufferLen, nil
	}
}

func (c *Conn) flush() {
	c.mu.Lock()
	if c.wbuf == nil || len(*c.wbuf) == 0 {
		c.mu.Unlock()
		return
	}

	fd := c.getFd()
	n, err := syscall.Write(int(fd), *c.wbuf)
	if err != nil {
		if errors.Is(err, unix.EAGAIN) {
			c.mu.Unlock()
			return
		}

		if errors.Is(err, syscall.EINTR) {
			c.mu.Unlock()
			return
		}

		unix.Close(int(fd))
		c.safeConns.Del(int(fd))
		c.mu.Unlock()
		return
	}
	if n == len(*c.wbuf) {
		putBytes(c.wbuf)
		c.wbuf = nil
		c.eventLoop.ResetRead(int(fd))
	} else if n > 0 {
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
