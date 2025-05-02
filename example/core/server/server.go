package main

import (
	"errors"
	"log/slog"
	"syscall"

	"github.com/antlabs/pulse/core"
	"golang.org/x/sys/unix"
)

func main() {

	as, err := core.Create(core.TriggerTypeEdge)
	if err != nil {
		return
	}
	// 使用示例文件描述符 0 (标准输入)
	go core.Accept("tcp", "127.0.0.1:8080", as)

	for {

		as.Poll(-1, func(fd int, state core.State, err error) {
			slog.Info("poll", "fd", fd, "state", state.String(), "err", err)
			if err != nil {
				if errors.Is(err, unix.EAGAIN) {
					return
				}
				unix.Close(fd)
				return
			}

			if state.IsRead() {
				var buf [1024]byte
				for {
					n, err := unix.Read(fd, buf[:])
					if err != nil {
						if errors.Is(err, unix.EAGAIN) {
							return
						}
						unix.Close(fd)
						return
					}
					if n > 0 {
						// TODO 这里没有处理 EAGAIN
						syscall.Write(fd, buf[:n])
					} else {
						slog.Info("read", "fd", fd, "state", state.String(), "err", err)
					}
				}
			}

			if state.IsWrite() {
				syscall.Write(fd, []byte("hello client"))
			}
		})
	}
}
