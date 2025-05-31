//go:build windows

package core

import "syscall"

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
