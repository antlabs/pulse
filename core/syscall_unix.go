//go:build !windows

package core

import "syscall"

func Write(fd int, p []byte) (n int, err error) {
	return syscall.Write(fd, p)
}

func Read(fd int, p []byte) (n int, err error) {
	return syscall.Read(fd, p)
}

func Close(fd int) error {
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
