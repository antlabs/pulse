package pulse

import (
	"fmt"
	"sync"

	"golang.org/x/sys/unix"
)

type Conn struct {
	fd   int
	wbuf *[]byte // write buffer
	mu   sync.Mutex
}

func newConn(fd int) *Conn {
	return &Conn{
		fd: fd,
	}
}

func (c *Conn) Write(data []byte) (int, error) {
	c.mu.Lock()
	if c.wbuf == nil {
		n, err := unix.Write(c.fd, data)
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

	n, err := unix.Write(c.fd, *c.wbuf)
	if n == len(*c.wbuf) {
		c.mu.Lock()
		c.wbuf = nil
		// TODO: release memory
		c.mu.Unlock()
	}

	return n, err
}

// handleData 处理数据的逻辑
func handleData[T any](c *Conn, options *Options[T], rawData []byte) {
	var data T

	// 如果配置了解码器，则尝试解码
	if options.decoder != nil {
		decodedData, err := options.decoder.Decode(rawData)
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

	// 调用 OnData 回调
	options.callback.OnData(c, data)
}
