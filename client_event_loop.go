package pulse

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/antlabs/pulse/core"
)

type ClientEventLoop struct {
	*MultiEventLoop
	next     uint32                // 轮询计数器
	conns    *core.SafeConns[Conn] // 每个事件循环的连接管理器
	callback Callback              // 回调函数
	ctx      context.Context       // 上下文
}

func NewClientEventLoop(ctx context.Context, opts ...func(*Options)) *ClientEventLoop {
	multiLoop, err := NewMultiEventLoop(ctx, opts...)
	if err != nil {
		panic(err)
	}

	// 初始化连接管理器
	conns := core.SafeConns[Conn]{}
	conns.Init(core.GetMaxFd())

	return &ClientEventLoop{
		MultiEventLoop: multiLoop,
		conns:          &conns,
		callback:       multiLoop.options.callback,
		ctx:            ctx,
	}
}

func (loop *ClientEventLoop) RegisterConn(conn net.Conn) error {
	// 1. 获取文件描述符
	fd, err := core.GetFdFromConn(conn)
	if err != nil {
		return fmt.Errorf("failed to get fd from connection: %w", err)
	}

	// 2. 关闭原始连接（因为我们要使用文件描述符）
	if err := conn.Close(); err != nil {
		return fmt.Errorf("failed to close original connection: %w", err)
	}

	// 3. 选择事件循环（轮询分配）
	eventLoopIndex := loop.selectEventLoop()
	eventLoop := loop.MultiEventLoop.eventLoops[eventLoopIndex]

	// 4. 创建新连接
	connInstance := loop.createConn(fd, loop.conns, eventLoop)

	// 5. 添加到连接管理器
	loop.conns.Add(fd, connInstance)

	// 6. 调用回调函数
	if loop.callback != nil {
		loop.callback.OnOpen(connInstance)
	}

	// 7. 添加到事件循环
	return eventLoop.AddRead(fd)
}

// selectEventLoop 选择事件循环（轮询分配）
func (loop *ClientEventLoop) selectEventLoop() int {
	return int(atomic.AddUint32(&loop.next, 1) % uint32(len(loop.MultiEventLoop.eventLoops)))
}

// createConn 创建连接实例
func (loop *ClientEventLoop) createConn(fd int, safeConns *core.SafeConns[Conn], eventLoop core.PollingApi) *Conn {
	return newConn(
		fd,
		safeConns,
		loop.MultiEventLoop.localTask,
		TaskTypeInEventLoop,
		eventLoop,
		loop.MultiEventLoop.options.eventLoopReadBufferSize,
		loop.MultiEventLoop.options.flowBackPressureRemoveRead,
	)
}

func (loop *ClientEventLoop) Serve() {
	n := len(loop.MultiEventLoop.eventLoops)
	var wg sync.WaitGroup
	wg.Add(n)
	defer wg.Wait()

	for i := 0; i < n; i++ {
		go func(idx int) {
			defer wg.Done()
			buf := make([]byte, loop.MultiEventLoop.options.eventLoopReadBufferSize)
			for {
				select {
				case <-loop.ctx.Done():
					return
				default:
				}
				_, err := loop.MultiEventLoop.eventLoops[idx].Poll(0, func(fd int, state core.State, pollErr error) {
					c := loop.conns.GetUnsafe(fd)
					if pollErr != nil {
						if c != nil {
							c.Close()
							if loop.callback != nil {
								loop.callback.OnClose(c, pollErr)
							}
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
							if loop.callback != nil {
								loop.callback.OnClose(c, err)
							}
							return
						}
						if n == 0 {
							c.Close()
							if loop.callback != nil {
								loop.callback.OnClose(c, nil)
							}
							return
						}
						if loop.callback != nil {
							loop.callback.OnData(c, buf[:n])
						}
					}
				})
				if err != nil {
					break
				}
			}
		}(i)
	}
}
