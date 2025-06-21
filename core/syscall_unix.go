//go:build !windows

package core

import (
	"errors"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

func Write(fd int, p []byte) (n int, err error) {
	return syscall.Write(fd, p)
}

func Read(fd int, p []byte) (n int, err error) {
	return syscall.Read(fd, p)
}

func Close(fd int) error {
	if fd <= 0 {
		return nil
	}
	return syscall.Close(fd)
}

func SetNoDelay(fd int, nodelay bool) error {
	if nodelay {
		return syscall.SetsockoptInt(
			fd,
			syscall.IPPROTO_TCP,
			syscall.TCP_NODELAY,
			1,
		)
	}
	return syscall.SetsockoptInt(
		fd,
		syscall.IPPROTO_TCP,
		syscall.TCP_NODELAY,
		0,
	)
}

const (
	EAGAIN = syscall.EAGAIN
	EINTR  = syscall.EINTR
)

// 复制一份socket
func GetFdFromConn(c net.Conn) (newFd int, err error) {
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

func GetSendBufferSize(fd int) (int, error) {
	size, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_SNDBUF)
	if err != nil {
		return 0, err
	}
	return size, nil
}
