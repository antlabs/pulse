package pulse

import (
	"context"
	"net"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/antlabs/pulse/core"
)

type ClientEventLoop struct {
	pollers  []core.PollingApi
	conns    []*core.SafeConns[Conn]
	callback Callback
	options  Options
	next     uint32 // 用于轮询分配
	ctx      context.Context
}

func NewClientEventLoop(ctx context.Context, opts ...func(*Options)) *ClientEventLoop {
	var options Options
	for _, opt := range opts {
		opt(&options)
	}
	n := runtime.NumCPU()
	pollers := make([]core.PollingApi, n)
	conns := make([]*core.SafeConns[Conn], n)
	for i := 0; i < n; i++ {
		pollers[i], _ = core.Create(core.TriggerTypeEdge)
		conns[i] = &core.SafeConns[Conn]{}
		conns[i].Init(core.GetMaxFd())
	}
	return &ClientEventLoop{
		pollers:  pollers,
		conns:    conns,
		callback: options.callback,
		options:  options,
		ctx:      ctx,
	}
}

func (loop *ClientEventLoop) RegisterConn(conn net.Conn) error {
	fd, err := core.GetFdFromConn(conn)
	if err != nil {
		return err
	}
	if err := conn.Close(); err != nil {
		return err
	}
	idx := atomic.AddUint32(&loop.next, 1) % uint32(len(loop.pollers))
	c := newConn(fd, loop.conns[idx], nil, TaskTypeInEventLoop, loop.pollers[idx], 4096, false)
	loop.conns[idx].Add(fd, c)
	loop.callback.OnOpen(c)
	return loop.pollers[idx].AddRead(fd)
}

func (loop *ClientEventLoop) Serve() {
	n := len(loop.pollers)
	var wg sync.WaitGroup
	wg.Add(n)
	defer wg.Wait()

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			buf := make([]byte, loop.options.eventLoopReadBufferSize)
			for {
				select {
				case <-loop.ctx.Done():
					return
				default:
				}
				_, err := loop.pollers[idx].Poll(0, func(fd int, state core.State, pollErr error) {
					c := loop.conns[idx].GetUnsafe(fd)
					if pollErr != nil {
						if c != nil {
							c.Close()
							loop.callback.OnClose(c, pollErr)
						}
						return
					}
					if c == nil {
						return
					}
					if state.IsRead() {
						c.mu.Lock()
						n, err := core.Read(fd, buf)
						c.mu.Unlock()
						if err != nil {
							c.Close()
							loop.callback.OnClose(c, err)
							return
						}
						if n == 0 {
							c.Close()
							loop.callback.OnClose(c, nil)
							return
						}
						loop.callback.OnData(c, buf[:n])
					}
				})
				if err != nil {
					break
				}
			}
		}(i)
	}
}
