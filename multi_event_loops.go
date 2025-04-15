package pulse

import (
	"errors"
	"log/slog"
	"runtime"

	"github.com/antlabs/pulse/core"
	"golang.org/x/sys/unix"
)

type MultiEventLoop[T any] struct {
	eventLoops []core.PollingApi
	options    Options[T]
}

func NewMultiEventLoop[T any](options ...Options[T]) (e *MultiEventLoop[T], err error) {
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

func (e *MultiEventLoop[T]) Loop() {
	var safeConns safeConns[Conn]
	safeConns.init()

	for _, eventLoop := range e.eventLoops {
		eventLoop := eventLoop
		go func() {
			for {
				eventLoop.Poll(-10, func(fd int, state core.State, err error) {

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
						c = newConn(fd)
						safeConns.Add(fd, c)
					}

					if state.IsRead() {
						for {
							n, err := unix.Read(fd, *c.rbuf)
							if err != nil {
								if errors.Is(err, unix.EAGAIN) {
									return
								}
								unix.Close(fd)
								safeConns.Del(fd)
								return
							}

							_ = n
							handleData[T](c, &e.options, *c.rbuf)
						}
					}

					if state.IsWrite() {
						unix.Write(fd, []byte("hello client"))
					}
				})
			}
		}()
	}
}

func (e *MultiEventLoop[T]) Free() {
	for _, eventLoop := range e.eventLoops {
		eventLoop.Free()
	}
}
