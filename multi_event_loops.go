package pulse

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/antlabs/pulse/core"
	"github.com/antlabs/pulse/task/driver"
	_ "github.com/antlabs/pulse/task/stream"
	_ "github.com/antlabs/pulse/task/stream2"
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

	if m.options.eventLoopReadBufferSize == 0 {
		m.options.eventLoopReadBufferSize = defEventLoopReadBufferSize
	}
}

func NewMultiEventLoop[T any](ctx context.Context, options ...func(*Options[T])) (e *MultiEventLoop[T], err error) {
	eventLoops := make([]core.PollingApi, runtime.NumCPU())

	var c driver.Conf
	c.Log = slog.Default()
	e = &MultiEventLoop[T]{
		eventLoops: eventLoops,
	}

	for _, option := range options {
		option(&e.options)
	}

	e.initDefaultSetting()
	for i := 0; i < runtime.NumCPU(); i++ {
		eventLoops[i], err = core.Create(e.options.triggerType)
		if err != nil {
			return nil, err
		}
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: e.options.level})))
	e.localTask = newSelectTask(ctx, e.options.task.initCount, e.options.task.min, e.options.task.max, &c)

	return e, nil
}

func (e *MultiEventLoop[T]) ListenAndServe(addr string) error {
	slog.Debug("listenAndServe", "addr", addr)
	var safeConns safeConns[Conn]
	safeConns.init(maxFd)

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
			rbuf := make([]byte, 1024*4)

			for {
				eventLoop.Poll(0, func(fd int, state core.State, err error) {

					c := safeConns.Get(fd)
					// slog.Debug("poll", "fd", fd, "state", state, "err", err)
					if err != nil {
						if errors.Is(err, core.EAGAIN) {
							return
						}
						if c != nil {
							c.Close()
						}
						return
					}

					if c == nil {
						c = newConn(fd, &safeConns, e.localTask, e.options.taskType, eventLoop)
						safeConns.Add(fd, c)
					}

					if c.needFlush() && state.IsWrite() {
						c.flush()
					}
					if state.IsRead() {
						for {
							// 循环读取数据
							c.mu.Lock()
							n, err := core.Read(c.getFd(), rbuf)
							c.mu.Unlock()
							if err != nil {
								// EAGAIN表示没有数据
								if errors.Is(err, core.EAGAIN) {
									return
								}

								if errors.Is(err, syscall.EINTR) {
									continue
								}
								// 如果不是这个错误直接关闭连接
								c.Close()
								return
							}

							if n == 0 {
								slog.Info("read 0 bytes", "fd", fd, "state", state.String(), "err", err)
								c.Close()
								break
							}

							handleData(c, &e.options, rbuf[:n])
						}
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
