package pulse

import (
	"context"
	"errors"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/antlabs/pulse/core"
	"github.com/antlabs/task/task/driver"
	_ "github.com/antlabs/task/task/stream"
	_ "github.com/antlabs/task/task/stream2"
)

type MultiEventLoop struct {
	eventLoops []core.PollingApi
	options    Options
	localTask  selectTasks
}

func (m *MultiEventLoop) initDefaultSetting() {

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

	if m.options.maxSocketReadTimes == 0 {
		if m.options.triggerType == TriggerTypeLevel {
			// 水平触发模式下使用默认值
			m.options.maxSocketReadTimes = defMaxSocketReadTimes
		} else {
			// 边缘触发模式下不限制读取次数
			m.options.maxSocketReadTimes = -1
		}
		m.options.maxSocketReadTimes = defMaxSocketReadTimes
	}
}

func NewMultiEventLoop(ctx context.Context, options ...func(*Options)) (e *MultiEventLoop, err error) {
	eventLoops := make([]core.PollingApi, runtime.NumCPU())

	var c driver.Conf
	c.Log = slog.Default()
	e = &MultiEventLoop{
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

func (e *MultiEventLoop) ListenAndServe(addr string) error {
	slog.Debug("listenAndServe", "addr", addr)
	var safeConns safeConns[Conn]
	safeConns.init(core.GetMaxFd())

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
		// 统计每个eventLoop的连接数
		count := make([]int, len(e.eventLoops))
		for i := 0; ; i++ {
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
			if err := c.Close(); err != nil {
				log.Printf("failed to close connection: %v", err)
			}

			// 轮询分配到eventLoop
			index := i % len(e.eventLoops)
			// index := fd % len(e.eventLoops)
			count[index]++

			c2 := newConn(fd, &safeConns, e.localTask, e.options.taskType, e.eventLoops[index])
			safeConns.Add(fd, c2)
			e.options.callback.OnOpen(c2)
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

			rbuf := make([]byte, e.options.eventLoopReadBufferSize)
			for {
				if _, err := eventLoop.Poll(0, func(fd int, state core.State, err error) {

					c := safeConns.GetUnsafe(fd)
					// c := safeConns.Get(fd)
					// slog.Debug("poll", "fd", fd, "state", state, "err", err)
					if err != nil {
						if errors.Is(err, core.EAGAIN) {
							return
						}
						if c != nil {
							c.Close()
							e.options.callback.OnClose(c, err)
						}
						return
					}

					if c == nil {
						panic("c is nil")
					}

					if state.IsWrite() && c.needFlush() {
						c.flush()
					}
					if state.IsRead() {
						e.doRead(c, rbuf)
					}

				}); err != nil {
					log.Printf("eventLoop.Poll error: %v", err)
				}

			}
		}()
	}
	return nil
}

func (e *MultiEventLoop) doRead(c *Conn, rbuf []byte) {
	for i := 0; ; i++ {
		if e.options.maxSocketReadTimes > 0 && i >= e.options.maxSocketReadTimes {
			return
		}

		// 循环读取数据
		c.mu.Lock()
		n, err := core.Read(c.getFd(), rbuf)
		c.mu.Unlock()
		if err != nil {
			if errors.Is(err, core.EINTR) {
				continue
			}

			// EAGAIN表示没有数据
			if errors.Is(err, core.EAGAIN) {
				return
			}

			// 如果不是这个错误直接关闭连接
			e.options.callback.OnClose(c, err)
			c.Close()
			return
		}

		if n == 0 {
			// 如果不是这个错误直接关闭连接
			c.Close()
			e.options.callback.OnClose(c, io.EOF)
			return
		}
		if n > 0 {
			handleData(c, &e.options, rbuf[:n])
		}

		// https://man7.org/linux/man-pages/man7/epoll.7.html
		// Do I need to continuously read/write a file descriptor until
		// EAGAIN when using the EPOLLET flag (edge-triggered behavior)?

		// Receiving an event from epoll_wait(2) should suggest to you
		// that such file descriptor is ready for the requested I/O
		// operation.  You must consider it ready until the next
		// (nonblocking) read/write yields EAGAIN.  When and how you will
		// use the file descriptor is entirely up to you.

		// For packet/token-oriented files (e.g., datagram socket,
		// terminal in canonical mode), the only way to detect the end of
		// the read/write I/O space is to continue to read/write until
		// EAGAIN.

		// For stream-oriented files (e.g., pipe, FIFO, stream socket),
		// the condition that the read/write I/O space is exhausted can
		// also be detected by checking the amount of data read from /
		// written to the target file descriptor.  For example, if you
		// call read(2) by asking to read a certain amount of data and
		// read(2) returns a lower number of bytes, you can be sure of
		// having exhausted the read I/O space for the file descriptor.
		// The same is true when writing using write(2).  (Avoid this
		// latter technique if you cannot guarantee that the monitored
		// file descriptor always refers to a stream-oriented file.)
		if n < len(rbuf) {
			break
		}
	}
}

func (e *MultiEventLoop) Free() {
	for _, eventLoop := range e.eventLoops {
		eventLoop.Free()
	}
}
