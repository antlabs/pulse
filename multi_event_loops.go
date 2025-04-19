package pulse

import (
	"errors"
	"log/slog"
	"net"
	"runtime"

	"github.com/antlabs/pulse/core"
	"golang.org/x/sys/unix"
)

type MultiEventLoop[T any] struct {
	eventLoops []core.PollingApi
	options    Options[T]
}

func NewMultiEventLoop[T any](options ...func(*Options[T])) (e *MultiEventLoop[T], err error) {
	eventLoops := make([]core.PollingApi, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		eventLoops[i], err = core.Create()
		if err != nil {
			return nil, err
		}
	}

	return &MultiEventLoop[T]{
		eventLoops: eventLoops,
	}, nil
}

func (e *MultiEventLoop[T]) ListenAndServe(addr string) error {
	var safeConns safeConns[Conn]
	safeConns.init()

	_, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	for _, eventLoop := range e.eventLoops {
		eventLoop := eventLoop
		go func() {
			for {
				eventLoop.Poll(-1, func(fd int, state core.State, err error) {

					slog.Info("poll", "fd", fd, "state", state.String(), "err", err)
					if err != nil {
						if errors.Is(err, unix.EAGAIN) {
							return
						}
						unix.Close(fd)
						safeConns.Del(fd)
						return
					}

					c := safeConns.Get(fd)
					if c == nil {
						c = newConn(fd, &safeConns)
						safeConns.Add(fd, c)
					}

					if state.IsRead() {
						for {
							// 循环读取数据
							n, err := unix.Read(fd, *c.rbuf)
							if err != nil {
								// EAGAIN表示没有数据
								if errors.Is(err, unix.EAGAIN) {
									return
								}
								// 如果不是这个错误直接关闭连接
								unix.Close(fd)
								safeConns.Del(fd)
								return
							}

							handleData[T](c, &e.options, (*c.rbuf)[:n])
						}
					}

					if c.needFlush() && state.IsWrite() {
						c.flush()
					}

				})
			}
		}()
	}
	return nil
}

func (e *MultiEventLoop[T]) Free() {
	for _, eventLoop := range e.eventLoops {
		eventLoop.Free()
	}
}
