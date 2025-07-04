package core

import (
	"net"
	"strings"
	"time"
)

// 枚举变量状态有可写，写读
type State uint32

const (
	// 可写
	WRITE State = 1 << iota
	// 可读
	READ
)

// 水平触发还是边缘触发
type TriggerType uint32

const (
	TriggerTypeLevel TriggerType = iota
	TriggerTypeEdge
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
	DelRead(fd int) error
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

	fd, err = GetFdFromConn(c)
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

		fd, err := GetFdFromConn(c)
		if err != nil {
			return err
		}

		err = e.AddRead(fd)
		if err != nil {
			return err
		}
	}

}
