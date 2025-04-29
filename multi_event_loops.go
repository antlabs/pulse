package pulse

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/antlabs/pulse/core"
	"github.com/antlabs/pulse/task/driver"
	_ "github.com/antlabs/pulse/task/io"
	_ "github.com/antlabs/pulse/task/stream"
	_ "github.com/antlabs/pulse/task/stream2"
	"golang.org/x/sys/unix"
)

type MultiEventLoop[T any] struct {
	eventLoops []core.PollingApi
	options    Options[T]
	localTask  selectTasks
	ctx        context.Context
}

func (m *MultiEventLoop[T]) initDefaultSetting() {

	if m.options.level == 0 {
		m.options.level = slog.LevelError //
	}
	if m.options.task.min == 0 {
		m.options.task.min = defTaskMin
	}

	if m.options.task.max == 0 {
		m.options.task.max = defTaskMax
	}

	if m.options.task.initCount == 0 {
		m.options.task.initCount = defTaskInitCount
	}
}

func NewMultiEventLoop[T any](ctx context.Context, options ...func(*Options[T])) (e *MultiEventLoop[T], err error) {
	eventLoops := make([]core.PollingApi, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		eventLoops[i], err = core.Create()
		if err != nil {
			return nil, err
		}
	}

	var c driver.Conf
	c.Log = slog.Default()
	e = &MultiEventLoop[T]{
		eventLoops: eventLoops,
	}
	e.initDefaultSetting()

	for _, option := range options {
		option(&e.options)
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: e.options.level})))
	e.localTask = newSelectTask(ctx, e.options.task.initCount, e.options.task.min, e.options.task.max, &c)

	return e, nil
}

func (e *MultiEventLoop[T]) ListenAndServe(addr string) error {
	slog.Debug("listenAndServe", "addr", addr)
	var safeConns safeConns[Conn]
	safeConns.init()

	l, err := net.Listen("tcp", addr)
	if err != nil {
		slog.Error("listen", "err", err)
		return err
	}

	var wg sync.WaitGroup
	wg.Add(1 + len(e.eventLoops))
	defer wg.Wait()

	go func() {
		defer wg.Done()
		for {
			c, err := l.Accept()
			if err != nil {
				// TODO 优化
				time.Sleep(time.Second * 1)
				continue
			}

			fd, err := core.GetFdFromConn(c)
			if err != nil {
				slog.Error("getFdFromConn", "err", err)
				continue
			}

			index := fd % len(e.eventLoops)
			err = e.eventLoops[index].AddRead(fd)
			if err != nil {
				slog.Error("addRead", "err", err)
				continue
			}
		}
	}()

	for _, eventLoop := range e.eventLoops {
		eventLoop := eventLoop
		go func() {
			defer wg.Done()
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
						c = newConn(fd, &safeConns, e.localTask)
						safeConns.Add(fd, c)
					}

					if state.IsRead() {
						if c.rbuf == nil {
							c.rbuf = getBytes(1024)
							*c.rbuf = (*c.rbuf)[:1024]
						}
						for {
							// 循环读取数据
							buf := *c.rbuf
							n, err := unix.Read(fd, buf)
							if err != nil {
								// EAGAIN表示没有数据
								if errors.Is(err, unix.EAGAIN) {
									putBytes(c.rbuf)
									c.rbuf = nil
									return
								}
								// 如果不是这个错误直接关闭连接
								unix.Close(fd)
								safeConns.Del(fd)
								return
							}

							if n == 0 {
								slog.Info("read 0 bytes", "fd", fd, "state", state.String(), "err", err)
								unix.Close(fd)
								safeConns.Del(fd)
								break
							}
							if n < 0 {
								panic("read n < 0")
							}

							handleData(c, &e.options, buf[:n])
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
