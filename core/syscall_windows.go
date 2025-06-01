//go:build windows

package core

import (
	"fmt"
	"net"
	"syscall"
)

func Write(fd int, p []byte) (n int, err error) {
	return syscall.Write(syscall.Handle(fd), p)
}

func Read(fd int, p []byte) (n int, err error) {
	return syscall.Read(syscall.Handle(fd), p)
}

func Close(fd int) error {
	return syscall.CloseHandle(syscall.Handle(fd))
}

const (
	// TODO 瞎写的值，为了windows下面编译通过
	EAGAIN = syscall.Errno(0x23)
	EINTR  = syscall.Errno(0x24)
)

func SetNoDelay(fd int, nodelay bool) error {
	return nil
}

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
