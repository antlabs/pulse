package core

import (
	"fmt"
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

// // 复制一份socket
// func GetFdFromConn(c net.Conn) (newFd int, err error) {
// 	sc, ok := c.(interface {
// 		SyscallConn() (syscall.RawConn, error)
// 	})
// 	if !ok {
// 		return 0, errors.New("RawConn Unsupported")
// 	}
// 	rc, err := sc.SyscallConn()
// 	if err != nil {
// 		return 0, errors.New("RawConn Unsupported")
// 	}

// 	err = rc.Control(func(fd uintptr) {
// 		newFd = int(fd)
// 	})
// 	if err != nil {
// 		return 0, err
// 	}

// 	return duplicateSocket(int(newFd))
// }

// func duplicateSocket(socketFD int) (int, error) {
// 	return unix.Dup(socketFD)
// }

func GetFdFromConn(conn net.Conn) (fd int, err error) {
	// 类型断言为 *net.TCPConn 或其他具体类型
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return 0, fmt.Errorf("not a TCP connection")
	}

	// 获取底层的 *os.File
	file, err := tcpConn.File()
	if err != nil {
		return 0, err
	}
	defer file.Close() // 注意：Close 会复制文件描述符，避免影响原连接

	// 获取文件描述符
	return int(file.Fd()), nil
}
