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
		n, err := unix.Write(int(c.fd), data)
		if err == unix.EAGAIN {
			*c.wbuf = append(*c.wbuf, data[n:]...)
			c.mu.Unlock()
			return 0, nil
		}
		c.mu.Unlock()
		return n, err
	}

	*c.wbuf = append(*c.wbuf, data...)
	c.mu.Unlock()

	n, err := unix.Write(int(c.fd), *c.wbuf)
	if n == len(*c.wbuf) {
		c.mu.Lock()
		c.wbuf = nil
		// TODO: release memory
		c.mu.Unlock()
	}

	return n, err
}

func (c *Conn) needFlush() bool {
	return c.wbuf != nil && len(*c.wbuf) > 0
}

func (c *Conn) flush() {
	c.mu.Lock()
	if c.wbuf == nil {
		c.mu.Unlock()
		return
	}

	fd := atomic.LoadInt64(&c.fd)
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
		c.wbuf = nil
		// TODO: release memory
	} else if n > 0 {
		copy(*c.wbuf, (*c.wbuf)[n:])
		*c.wbuf = (*c.wbuf)[:len(*c.wbuf)-n]
	}
	c.mu.Unlock()
}

// handleData 处理数据的逻辑
func handleData[T any](c *Conn, options *Options[T], rawData []byte) {
	var data T

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
		raw, ok := any(rawData).(T)
		if !ok {
			fmt.Println("Type assertion failed for raw data")
			return
		}
		data = raw
	}

	// 进入协程池
	c.task.AddTask(&c.mu, func() bool {
		options.callback.OnData(c, data)
		return true
	})
}
