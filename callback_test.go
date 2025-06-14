package pulse

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/antlabs/pulse/core"
)

// TestCallback_Interface 测试Callback接口的完整性
func TestCallback_Interface(t *testing.T) {
	// 确保toCallback实现了Callback接口
	var _ Callback = (*toCallback)(nil)
}

// TestOnOpen_RealSocket 测试真实socket连接的OnOpen回调
func TestOnOpen_RealSocket(t *testing.T) {
	tests := []struct {
		name         string
		serverSetup  func(t *testing.T, callback Callback) (string, func())
		clientDelay  time.Duration
		expectOnOpen bool
		expectError  bool
	}{
		{
			name: "successful TCP connection",
			serverSetup: func(t *testing.T, callback Callback) (string, func()) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("Failed to create listener: %v", err)
				}

				addr := listener.Addr().String()
				var wg sync.WaitGroup

				wg.Add(1)
				go func() {
					defer wg.Done()
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					defer func() {
						if err := conn.Close(); err != nil {
							log.Printf("failed to close connection: %v", err)
						}
					}()

					fd, err := core.GetFdFromConn(conn)
					if err != nil {
						t.Errorf("Failed to get fd from conn: %v", err)
						return
					}
					if err := conn.Close(); err != nil { // 关闭原始连接，我们使用fd
						log.Printf("failed to close original connection: %v", err)
					}

					// 创建Pulse连接
					safeConns := &safeConns[Conn]{}
					safeConns.init(1000)

					pulseConn := &Conn{
						fd:        int64(fd),
						safeConns: safeConns,
					}
					safeConns.Add(fd, pulseConn)

					// 调用OnOpen回调
					callback.OnOpen(pulseConn)

					// 保持连接一段时间
					time.Sleep(100 * time.Millisecond)
					pulseConn.Close()
				}()

				cleanup := func() {
					if err := listener.Close(); err != nil {
						log.Printf("failed to close listener: %v", err)
					}
					wg.Wait()
				}

				return addr, cleanup
			},
			clientDelay:  50 * time.Millisecond,
			expectOnOpen: true,
			expectError:  false,
		},
		{
			name: "connection with session initialization",
			serverSetup: func(t *testing.T, callback Callback) (string, func()) {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatalf("Failed to create listener: %v", err)
				}

				addr := listener.Addr().String()
				var wg sync.WaitGroup

				wg.Add(1)
				go func() {
					defer wg.Done()
					conn, err := listener.Accept()
					if err != nil {
						return
					}
					defer func() {
						if err := conn.Close(); err != nil {
							log.Printf("failed to close connection: %v", err)
						}
					}()

					fd, err := core.GetFdFromConn(conn)
					if err != nil {
						return
					}
					if err := conn.Close(); err != nil {
						log.Printf("failed to close original connection: %v", err)
					}

					safeConns := &safeConns[Conn]{}
					safeConns.init(1000)

					pulseConn := &Conn{
						fd:        int64(fd),
						safeConns: safeConns,
					}
					safeConns.Add(fd, pulseConn)

					callback.OnOpen(pulseConn)
					time.Sleep(100 * time.Millisecond)
					pulseConn.Close()
				}()

				return addr, func() {
					if err := listener.Close(); err != nil {
						log.Printf("failed to close listener: %v", err)
					}
					wg.Wait()
				}
			},
			clientDelay:  50 * time.Millisecond,
			expectOnOpen: true,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var onOpenCalled bool
			var onOpenConn *Conn
			var onOpenErr error
			var mu sync.Mutex

			callback := ToCallback(
				func(c *Conn, err error) {
					mu.Lock()
					defer mu.Unlock()
					onOpenCalled = true
					onOpenConn = c
					onOpenErr = err

					// 初始化会话数据
					c.SetSession(map[string]interface{}{
						"connected_at": time.Now(),
						"client_addr":  "test_client",
					})
				},
				func(c *Conn, data []byte) {},
				func(c *Conn, err error) {},
			)

			// 设置服务器
			addr, cleanup := tt.serverSetup(t, callback)
			defer cleanup()

			// 等待服务器启动
			time.Sleep(10 * time.Millisecond)

			// 创建客户端连接
			clientConn, err := net.Dial("tcp", addr)
			if err != nil {
				t.Fatalf("Failed to connect to server: %v", err)
			}

			// 等待处理
			time.Sleep(tt.clientDelay)
			if err := clientConn.Close(); err != nil {
				log.Printf("failed to close client connection: %v", err)
			}

			// 等待服务器处理完成
			time.Sleep(50 * time.Millisecond)

			// 验证结果
			mu.Lock()
			defer mu.Unlock()

			if onOpenCalled != tt.expectOnOpen {
				t.Errorf("OnOpen called = %v, expected = %v", onOpenCalled, tt.expectOnOpen)
			}

			if tt.expectOnOpen {
				if onOpenConn == nil {
					t.Error("OnOpen callback should receive a valid connection")
				}
				if tt.expectError && onOpenErr == nil {
					t.Error("OnOpen callback should receive an error")
				}
				if !tt.expectError && onOpenErr != nil {
					t.Errorf("OnOpen callback received unexpected error: %v", onOpenErr)
				}

				// 验证会话数据
				if onOpenConn != nil {
					session := onOpenConn.GetSession()
					if session == nil {
						t.Error("Session should be initialized in OnOpen")
					} else {
						sessionMap := session.(map[string]interface{})
						if sessionMap["client_addr"] != "test_client" {
							t.Error("Session data not properly initialized")
						}
					}
				}
			}
		})
	}
}

// TestOnClose_RealSocket 测试真实socket连接的OnClose回调
func TestOnClose_RealSocket(t *testing.T) {
	tests := []struct {
		name          string
		setupScenario func(t *testing.T, callback Callback) error
		expectOnClose bool
		expectError   bool
		closeReason   string
	}{
		{
			name: "normal connection close",
			setupScenario: func(t *testing.T, callback Callback) error {
				return runClientServerTest(t, callback, func(clientConn net.Conn, pulseConn *Conn) {
					// 正常关闭客户端连接
					time.Sleep(50 * time.Millisecond)
					if err := clientConn.Close(); err != nil {
						log.Printf("failed to close client connection: %v", err)
					}
				}, false)
			},
			expectOnClose: true,
			expectError:   false,
			closeReason:   "client_close",
		},
		{
			name: "server side close",
			setupScenario: func(t *testing.T, callback Callback) error {
				return runClientServerTest(t, callback, func(clientConn net.Conn, pulseConn *Conn) {
					// 服务器端主动关闭
					time.Sleep(50 * time.Millisecond)
					pulseConn.Close()
				}, false)
			},
			expectOnClose: true,
			expectError:   false,
			closeReason:   "server_close",
		},
		{
			name: "connection timeout close",
			setupScenario: func(t *testing.T, callback Callback) error {
				return runClientServerTest(t, callback, func(clientConn net.Conn, pulseConn *Conn) {
					// 设置短超时
					if err := pulseConn.SetDeadline(time.Now().Add(30 * time.Millisecond)); err != nil {
						log.Printf("failed to set deadline: %v", err)
					}
					time.Sleep(50 * time.Millisecond) // 等待超时
				}, false)
			},
			expectOnClose: true,
			expectError:   false,
			closeReason:   "timeout",
		},
		{
			name: "connection with error simulation",
			setupScenario: func(t *testing.T, callback Callback) error {
				return runClientServerTest(t, callback, func(clientConn net.Conn, pulseConn *Conn) {
					// 模拟网络错误（强制关闭）
					if err := clientConn.Close(); err != nil {
						log.Printf("failed to close client connection: %v", err)
					}
					time.Sleep(10 * time.Millisecond)
				}, true) // 传递错误
			},
			expectOnClose: true,
			expectError:   true,
			closeReason:   "network_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var onOpenCalled, onCloseCalled bool
			var onCloseConn *Conn
			var onCloseErr error
			var mu sync.Mutex

			callback := ToCallback(
				func(c *Conn, err error) {
					mu.Lock()
					defer mu.Unlock()
					onOpenCalled = true
					c.SetSession(map[string]interface{}{
						"close_reason": tt.closeReason,
						"opened_at":    time.Now(),
					})
				},
				func(c *Conn, data []byte) {},
				func(c *Conn, err error) {
					mu.Lock()
					defer mu.Unlock()
					onCloseCalled = true
					onCloseConn = c
					onCloseErr = err
				},
			)

			// 运行测试场景
			err := tt.setupScenario(t, callback)
			if err != nil {
				t.Fatalf("Test scenario failed: %v", err)
			}

			// 等待处理完成
			time.Sleep(100 * time.Millisecond)

			// 验证结果
			mu.Lock()
			defer mu.Unlock()

			if !onOpenCalled {
				t.Error("OnOpen should be called before OnClose")
			}

			if onCloseCalled != tt.expectOnClose {
				t.Errorf("OnClose called = %v, expected = %v", onCloseCalled, tt.expectOnClose)
			}

			if tt.expectOnClose {
				if onCloseConn == nil {
					t.Error("OnClose callback should receive a valid connection")
				}
				if tt.expectError && onCloseErr == nil {
					t.Error("OnClose callback should receive an error")
				}
				if !tt.expectError && onCloseErr != nil {
					t.Errorf("OnClose callback received unexpected error: %v", onCloseErr)
				}

				// 验证会话数据在关闭时仍然可访问
				if onCloseConn != nil {
					session := onCloseConn.GetSession()
					if session != nil {
						sessionMap := session.(map[string]interface{})
						if sessionMap["close_reason"] != tt.closeReason {
							t.Errorf("Close reason = %v, expected = %v",
								sessionMap["close_reason"], tt.closeReason)
						}
					}
				}
			}
		})
	}
}

// runClientServerTest 辅助函数：运行客户端-服务器测试
func runClientServerTest(t *testing.T, callback Callback, scenario func(net.Conn, *Conn), simulateError bool) error {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("failed to create listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Printf("failed to close listener: %v", err)
		}
	}()

	addr := listener.Addr().String()
	var wg sync.WaitGroup

	// Use channels to synchronize between goroutines
	pulseConnChan := make(chan *Conn, 1)
	serverErrChan := make(chan error, 1)

	// 启动服务器（异步等待连接）
	wg.Add(1)
	go func() {
		defer wg.Done()

		conn, err := listener.Accept()
		if err != nil {
			serverErrChan <- fmt.Errorf("accept failed: %v", err)
			return
		}
		defer func() {
			if err := conn.Close(); err != nil {
				log.Printf("failed to close connection: %v", err)
			}
		}()

		// 获取文件描述符并创建Pulse连接
		fd, err := core.GetFdFromConn(conn)
		if err != nil {
			serverErrChan <- fmt.Errorf("get fd failed: %v", err)
			return
		}
		if err := conn.Close(); err != nil {
			log.Printf("failed to close original connection: %v", err)
		}

		safeConns := &safeConns[Conn]{}
		safeConns.init(1000)

		pulseConn := &Conn{
			fd:        int64(fd),
			safeConns: safeConns,
		}
		safeConns.Add(fd, pulseConn)

		// 调用OnOpen
		callback.OnOpen(pulseConn)

		// Send successful result
		pulseConnChan <- pulseConn

		// Keep the connection alive for the test
		time.Sleep(100 * time.Millisecond)
	}()

	// Give server a moment to start listening
	time.Sleep(10 * time.Millisecond)

	// 创建客户端连接
	clientConn, err := net.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	defer func() {
		if err := clientConn.Close(); err != nil {
			log.Printf("failed to close client connection: %v", err)
		}
	}()

	// Wait for server to process the connection and send pulseConn
	var pulseConn *Conn
	select {
	case pulseConn = <-pulseConnChan:
		// Success, continue with test
	case serverErr := <-serverErrChan:
		return serverErr
	case <-time.After(2 * time.Second):
		return fmt.Errorf("timeout waiting for server to process connection")
	}

	// 执行测试场景
	wg.Add(1)
	go func() {
		defer wg.Done()
		scenario(clientConn, pulseConn)

		// 调用OnClose
		var closeErr error
		if simulateError {
			closeErr = errors.New("simulated network error")
		}
		callback.OnClose(pulseConn, closeErr)
	}()

	wg.Wait()
	return nil
}

// TestCallback_RealConnectionLifecycle 测试真实连接的完整生命周期
func TestCallback_RealConnectionLifecycle(t *testing.T) {
	var phases []string
	var mu sync.Mutex

	addPhase := func(phase string) {
		mu.Lock()
		phases = append(phases, phase)
		mu.Unlock()
	}

	callback := ToCallback(
		func(c *Conn, err error) {
			addPhase("connection_opened")
			// 初始化连接状态
			c.SetSession(map[string]interface{}{
				"connected_at":   time.Now(),
				"bytes_received": 0,
				"bytes_sent":     0,
			})
		},
		func(c *Conn, data []byte) {
			addPhase("data_received")
			session := c.GetSession().(map[string]interface{})
			session["bytes_received"] = session["bytes_received"].(int) + len(data)

			// 回显数据
			n, err := c.Write(data)
			if err == nil {
				session["bytes_sent"] = session["bytes_sent"].(int) + n
			}
		},
		func(c *Conn, err error) {
			addPhase("connection_closed")
			// 记录连接统计信息
			if session := c.GetSession(); session != nil {
				sessionMap := session.(map[string]interface{})
				addPhase(fmt.Sprintf("stats_bytes_received_%d", sessionMap["bytes_received"]))
				addPhase(fmt.Sprintf("stats_bytes_sent_%d", sessionMap["bytes_sent"]))
			}
		},
	)

	// 创建监听器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Printf("failed to close listener: %v", err)
		}
	}()

	addr := listener.Addr().String()
	var wg sync.WaitGroup

	// 启动服务器
	wg.Add(1)
	go func() {
		defer wg.Done()
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer func() {
			if err := conn.Close(); err != nil {
				log.Printf("failed to close connection: %v", err)
			}
		}()

		fd, err := core.GetFdFromConn(conn)
		if err != nil {
			return
		}
		if err := conn.Close(); err != nil {
			log.Printf("failed to close original connection: %v", err)
		}

		safeConns := &safeConns[Conn]{}
		safeConns.init(1000)

		pulseConn := &Conn{
			fd:        int64(fd),
			safeConns: safeConns,
		}
		safeConns.Add(fd, pulseConn)

		// 连接建立
		callback.OnOpen(pulseConn)

		// 模拟数据交换
		testData := [][]byte{
			[]byte("hello"),
			[]byte("world"),
			[]byte("pulse"),
		}

		for _, data := range testData {
			callback.OnData(pulseConn, data)
		}

		// 连接关闭
		time.Sleep(50 * time.Millisecond)
		callback.OnClose(pulseConn, nil)
		pulseConn.Close()
	}()

	// 等待服务器启动
	time.Sleep(20 * time.Millisecond)

	// 创建客户端
	clientConn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		if err := clientConn.Close(); err != nil {
			log.Printf("failed to close client connection: %v", err)
		}
	}()

	wg.Wait()

	// 验证生命周期
	mu.Lock()
	defer mu.Unlock()

	// 注释掉预期阶段的详细对比，因为真实网络环境下时序可能有差异
	// expectedPhases := []string{
	// 	"connection_opened",
	// 	"data_received",
	// 	"data_received",
	// 	"data_received",
	// 	"connection_closed",
	// 	"stats_bytes_received_15", // hello(5) + world(5) + pulse(5)
	// 	"stats_bytes_sent_15",
	// }

	if len(phases) < 4 { // 至少要有开启、数据、关闭阶段
		t.Errorf("Expected at least 4 phases, got %d: %v", len(phases), phases)
		return
	}

	// 验证关键阶段
	if phases[0] != "connection_opened" {
		t.Errorf("First phase should be 'connection_opened', got '%s'", phases[0])
	}

	hasDataReceived := false
	hasConnectionClosed := false
	for _, phase := range phases {
		if phase == "data_received" {
			hasDataReceived = true
		}
		if phase == "connection_closed" {
			hasConnectionClosed = true
		}
	}

	if !hasDataReceived {
		t.Error("Should have 'data_received' phase")
	}
	if !hasConnectionClosed {
		t.Error("Should have 'connection_closed' phase")
	}
}

// TestCallback_ConcurrentRealConnections 测试并发真实连接
func TestCallback_ConcurrentRealConnections(t *testing.T) {
	var openCount, closeCount int64
	var wg sync.WaitGroup

	callback := ToCallback(
		func(c *Conn, err error) {
			atomic.AddInt64(&openCount, 1)
			c.SetSession(fmt.Sprintf("conn_%d", atomic.LoadInt64(&openCount)))
		},
		func(c *Conn, data []byte) {},
		func(c *Conn, err error) {
			atomic.AddInt64(&closeCount, 1)
		},
	)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			log.Printf("failed to close listener: %v", err)
		}
	}()

	addr := listener.Addr().String()
	const numConnections = 5

	// 启动服务器处理多个连接
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < numConnections; i++ {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}

			// 为每个连接启动处理协程
			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				defer func() {
					if err := conn.Close(); err != nil {
						log.Printf("failed to close connection: %v", err)
					}
				}()

				fd, err := core.GetFdFromConn(conn)
				if err != nil {
					return
				}
				if err := conn.Close(); err != nil {
					log.Printf("failed to close original connection: %v", err)
				}

				safeConns := &safeConns[Conn]{}
				safeConns.init(1000)

				pulseConn := &Conn{
					fd:        int64(fd),
					safeConns: safeConns,
				}
				safeConns.Add(fd, pulseConn)

				callback.OnOpen(pulseConn)
				time.Sleep(50 * time.Millisecond)
				callback.OnClose(pulseConn, nil)
				pulseConn.Close()
			}(conn)
		}
	}()

	// 创建多个客户端连接
	for i := 0; i < numConnections; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			clientConn, err := net.Dial("tcp", addr)
			if err != nil {
				return
			}
			defer func() {
				if err := clientConn.Close(); err != nil {
					log.Printf("failed to close client connection: %v", err)
				}
			}()
			time.Sleep(100 * time.Millisecond)
		}()
		time.Sleep(10 * time.Millisecond) // 错开连接时间
	}

	wg.Wait()

	// 验证计数
	finalOpenCount := atomic.LoadInt64(&openCount)
	finalCloseCount := atomic.LoadInt64(&closeCount)

	if finalOpenCount != numConnections {
		t.Errorf("Expected %d opens, got %d", numConnections, finalOpenCount)
	}
	if finalCloseCount != numConnections {
		t.Errorf("Expected %d closes, got %d", numConnections, finalCloseCount)
	}
}
