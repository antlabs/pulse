package pulse

import (
	"sync"

	"golang.org/x/sys/unix"
)

type Conn struct {
	fd   int
	wbuf *[]byte // write buffer
	mu   sync.Mutex
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
