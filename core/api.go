package core

import (
	"errors"
	"net"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// 枚举变量状态有可写，写读
type State uint32

const (
	// 可写
	WRITE State = 1 << iota
	// 可读
	READ
)

func (s State) String() string {
	var sb strings.Builder
	if s.IsRead() {
		sb.WriteString("READ")
	}
	if s.IsWrite() {
		sb.WriteString("WRITE")
	}
	return sb.String()
}

func (s State) IsWrite() bool {
	return s&WRITE != 0
}

func (s State) IsRead() bool {
	return s&READ != 0
}

type PollingApi interface {
	AddRead(fd int) error
	AddWrite(fd int) error
	ResetRead(fd int) error
	Del(fd int) error
	Poll(tv time.Duration, cb func(int, State, error)) (retVal int, err error)
	Free()
	Name() string
}

// dial 工具函数
func Dial(network, addr string, e PollingApi) (fd int, err error) {
	c, err := net.Dial(network, addr)
	if err != nil {
		return 0, err
	}

	fd, err = getFdFromConn(c)
	if err != nil {
		return 0, err
	}

	err = e.AddRead(fd)
	if err != nil {
		return 0, err
	}

	return fd, nil
}

// Accept 工具函数
func Accept(network, addr string, e PollingApi) error {
	l, err := net.Listen(network, addr)
	if err != nil {
		return err
	}

	for {
		c, err := l.Accept()
		if err != nil {
			time.Sleep(time.Second * 1)
			continue
		}

		fd, err := getFdFromConn(c)
		if err != nil {
			return err
		}

		err = e.AddRead(fd)
		if err != nil {
			return err
		}
	}

}

// 复制一份socket
func getFdFromConn(c net.Conn) (newFd int, err error) {
	sc, ok := c.(interface {
		SyscallConn() (syscall.RawConn, error)
	})
	if !ok {
		return 0, errors.New("RawConn Unsupported")
	}
	rc, err := sc.SyscallConn()
	if err != nil {
		return 0, errors.New("RawConn Unsupported")
	}

	err = rc.Control(func(fd uintptr) {
		newFd = int(fd)
	})
	if err != nil {
		return 0, err
	}

	return duplicateSocket(int(newFd))
}

func duplicateSocket(socketFD int) (int, error) {
	return unix.Dup(socketFD)
}
