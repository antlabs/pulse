package pulse

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/antlabs/pulse/task/driver"
	"golang.org/x/sys/unix"
)

type Conn struct {
	fd        int64
	rbuf      *[]byte // read buffer, 为了更精细控制内存使用量
	wbuf      *[]byte // write buffer, 为了理精细控制内存使用量
	mu        sync.Mutex
	safeConns *safeConns[Conn]
	task      driver.TaskExecutor
}

func (c *Conn) getFd() int {
	return int(atomic.LoadInt64(&c.fd))
}
func newConn(fd int, safeConns *safeConns[Conn], task selectTasks) *Conn {
	return &Conn{
		fd:        int64(fd),
		safeConns: safeConns,
		task:      task.newTask("stream2"),
	}
}

func (c *Conn) Write(data []byte) (int, error) {
	c.mu.Lock()
	if c.wbuf == nil {
		n, err := unix.Write(c.getFd(), data)
		if err == unix.EAGAIN {
			newBytes := getBytes(len(data))
			*newBytes = (*newBytes)[:len(data)]
			c.wbuf = newBytes
			// newBytes := make([]byte, len(data))
			// c.wbuf = &newBytes
			if n > 0 {
				// 部分写成功
				copy(*c.wbuf, data[:n])
				*c.wbuf = (*c.wbuf)[:n]
			} else {
				// 全部写失败
				copy(*c.wbuf, data)
				*c.wbuf = (*c.wbuf)[:len(data)]
			}
			c.mu.Unlock()
			return 0, nil
		}
		c.mu.Unlock()
		return n, err
	}

	*c.wbuf = append(*c.wbuf, data...)

	n, err := unix.Write(int(c.getFd()), *c.wbuf)
	if n == len(*c.wbuf) {
		putBytes(c.wbuf)
		c.wbuf = nil
	}
	c.mu.Unlock()

	return n, err
}

func (c *Conn) flush() {
	c.mu.Lock()
	if c.wbuf == nil || len(*c.wbuf) == 0 {
		c.mu.Unlock()
		return
	}

	fd := c.getFd()
	n, err := unix.Write(int(fd), *c.wbuf)
	if err != nil {
		if errors.Is(err, unix.EAGAIN) {
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
		newBytes = getBytes(len(rawData))
		copy(*newBytes, rawData)
		*newBytes = (*newBytes)[:len(rawData)]
		data = any(*newBytes).(T)
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
