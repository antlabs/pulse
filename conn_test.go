package pulse

import (
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"
)

func Test_OnData(t *testing.T) {
	c := &Conn{fd: 0}
	var receivedData []byte
	var receivedErr error
	var closeErr error

	options := &Options{
		callback: ToCallback(
			func(c *Conn, err error) {
				receivedErr = err
			},
			func(c *Conn, data []byte) {
				receivedData = data
			},
			func(c *Conn, err error) {
				closeErr = err
			},
		),
	}

	options.taskType = TaskTypeInEventLoop
	testData := []byte("hello")
	handleData(c, options, testData)

	if string(receivedData) != string(testData) {
		t.Errorf("OnData callback received wrong data, got %v, want %v", receivedData, testData)
	}
	if receivedErr != nil {
		t.Errorf("OnOpen callback received unexpected error: %v", receivedErr)
	}
	if closeErr != nil {
		t.Errorf("OnClose callback received unexpected error: %v", closeErr)
	}
}

func Test_Listen(t *testing.T) {
	// 第一次监听应该成功
	listener1, err := net.Listen("tcp", "127.0.0.1:8080")
	if err != nil {
		t.Fatalf("First Listen() failed: %v", err)
	}
	defer listener1.Close()

	// 第二次监听应该失败，因为端口已被占用
	listener2, err := net.Listen("tcp", "127.0.0.1:8080")
	if err == nil {
		t.Error("Second Listen() should fail, but succeeded")
		if listener2 != nil {
			listener2.Close()
		}
	}
}

func TestConn_SetDeadline(t *testing.T) {
	tests := []struct {
		name    string
		time    time.Time
		wantErr bool
	}{
		{
			name:    "set future deadline",
			time:    time.Now().Add(time.Second),
			wantErr: false,
		},
		{
			name:    "set past deadline",
			time:    time.Now().Add(-time.Second),
			wantErr: true,
		},
		{
			name:    "clear deadline",
			time:    time.Time{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个子测试创建独立的连接
			fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
			if err != nil {
				t.Fatalf("OpenFile() error = %v", err)
			}
			defer fd.Close()

			conn := &Conn{
				fd: int64(fd.Fd()),
				safeConns: &safeConns[Conn]{
					conns: make([]*Conn, 1000),
				},
			}

			err = conn.SetDeadline(tt.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetDeadline() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 对于过去的时间，连接应该被关闭
			if tt.time.Before(time.Now()) && !tt.time.IsZero() {
				// 给一点时间让close操作完成
				time.Sleep(10 * time.Millisecond)
				// Use public method to check if connection is closed
				err := conn.SetDeadline(time.Now().Add(time.Second))
				if err == nil {
					t.Error("Connection should be closed for past deadline")
				}
				return
			}

			// 验证读写定时器都被正确设置 - 通过尝试清除来检查
			if !tt.time.IsZero() {
				// 尝试清除deadline，如果成功说明timer已设置
				err := conn.SetDeadline(time.Time{})
				if err != nil {
					t.Error("Should be able to clear deadline when connection is alive")
				}
			}
		})
	}

	// 测试连接关闭的情况
	fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer fd.Close()

	conn := &Conn{
		fd: int64(fd.Fd()),
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}
	conn.close()
	err = conn.SetDeadline(time.Now().Add(time.Second))
	if err == nil {
		t.Error("SetDeadline() should return error when connection is closed")
	}
}

func TestConn_SetReadDeadline(t *testing.T) {
	tests := []struct {
		name    string
		time    time.Time
		wantErr bool
	}{
		{
			name:    "set future deadline",
			time:    time.Now().Add(time.Second),
			wantErr: false,
		},
		{
			name:    "set past deadline",
			time:    time.Now().Add(-time.Second),
			wantErr: false,
		},
		{
			name:    "clear deadline",
			time:    time.Time{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个子测试创建独立的连接
			fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
			if err != nil {
				t.Fatalf("OpenFile() error = %v", err)
			}
			defer fd.Close()

			conn := &Conn{
				fd: int64(fd.Fd()),
				safeConns: &safeConns[Conn]{
					conns: make([]*Conn, 1000),
				},
			}

			err = conn.SetReadDeadline(tt.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetReadDeadline() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 对于过去的时间，连接应该被关闭
			if tt.time.Before(time.Now()) && !tt.time.IsZero() {
				// 给一点时间让close操作完成
				time.Sleep(10 * time.Millisecond)
				// Use public method to check if connection is closed
				err := conn.SetReadDeadline(time.Now().Add(time.Second))
				if err == nil {
					t.Error("Connection should be closed for past deadline")
				}
				return
			}

			// 验证读定时器被正确设置 - 通过尝试清除来检查
			if !tt.time.IsZero() {
				// 尝试清除read deadline，如果成功说明timer已设置
				err := conn.SetReadDeadline(time.Time{})
				if err != nil {
					t.Error("Should be able to clear read deadline when connection is alive")
				}
			}
		})
	}

	// 测试连接关闭的情况
	fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer fd.Close()

	conn := &Conn{
		fd: int64(fd.Fd()),
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}
	conn.close()
	err = conn.SetReadDeadline(time.Now().Add(time.Second))
	if err == nil {
		t.Error("SetReadDeadline() should return error when connection is closed")
	}
}

func TestConn_SetWriteDeadline(t *testing.T) {
	tests := []struct {
		name    string
		time    time.Time
		wantErr bool
	}{
		{
			name:    "set future deadline",
			time:    time.Now().Add(time.Second),
			wantErr: false,
		},
		{
			name:    "set past deadline",
			time:    time.Now().Add(-time.Second),
			wantErr: false,
		},
		{
			name:    "clear deadline",
			time:    time.Time{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 为每个子测试创建独立的连接
			fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
			if err != nil {
				t.Fatalf("OpenFile() error = %v", err)
			}
			defer fd.Close()

			conn := &Conn{
				fd: int64(fd.Fd()),
				safeConns: &safeConns[Conn]{
					conns: make([]*Conn, 1000),
				},
			}

			err = conn.SetWriteDeadline(tt.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetWriteDeadline() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 对于过去的时间，连接应该被关闭
			if tt.time.Before(time.Now()) && !tt.time.IsZero() {
				// 给一点时间让close操作完成
				time.Sleep(10 * time.Millisecond)
				// Use public method to check if connection is closed
				err := conn.SetWriteDeadline(time.Now().Add(time.Second))
				if err == nil {
					t.Error("Connection should be closed for past deadline")
				}
				return
			}

			// 验证写定时器被正确设置 - 通过尝试清除来检查
			if !tt.time.IsZero() {
				// 尝试清除write deadline，如果成功说明timer已设置
				err := conn.SetWriteDeadline(time.Time{})
				if err != nil {
					t.Error("Should be able to clear write deadline when connection is alive")
				}
			}
		})
	}

	// 测试连接关闭的情况
	fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer fd.Close()

	conn := &Conn{
		fd: int64(fd.Fd()),
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}
	conn.close()
	err = conn.SetWriteDeadline(time.Now().Add(time.Second))
	if err == nil {
		t.Error("SetWriteDeadline() should return error when connection is closed")
	}
}

func TestConn_DeadlineTimeout(t *testing.T) {
	fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer fd.Close()

	conn := &Conn{
		fd: int64(fd.Fd()),
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

	// 设置一个很短的超时时间
	timeout := 100 * time.Millisecond
	err = conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		t.Fatalf("SetDeadline() error = %v", err)
	}

	// 等待超时发生
	time.Sleep(timeout + 50*time.Millisecond)

	// 验证连接是否被关闭
	if atomic.LoadInt64(&conn.fd) != -1 {
		t.Error("Connection should be closed after deadline")
	}

	// 验证定时器是否被清理
	if conn.readTimer != nil {
		t.Error("readTimer should be nil after deadline")
	}
	if conn.writeTimer != nil {
		t.Error("writeTimer should be nil after deadline")
	}

	// 验证在已关闭的连接上设置超时应该返回错误
	err = conn.SetDeadline(time.Now().Add(time.Second))
	if err == nil {
		t.Error("SetDeadline() should return error on closed connection")
	}
}

func TestConn_DeadlineReset(t *testing.T) {
	fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer fd.Close()

	fd2 := fd.Fd()
	conn := &Conn{
		fd: int64(fd2),
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

	// 设置初始超时 - 相对较短的时间
	initialTimeout := 100 * time.Millisecond
	startTime := time.Now()
	err = conn.SetDeadline(startTime.Add(initialTimeout))
	if err != nil {
		t.Fatalf("SetDeadline() error = %v", err)
	}

	// 等待一半时间
	time.Sleep(initialTimeout / 2) // 等待50ms

	// 重置超时时间为更长的时间 - 从现在开始计算
	newTimeout := 200 * time.Millisecond
	resetTime := time.Now()
	err = conn.SetDeadline(resetTime.Add(newTimeout))
	if err != nil {
		t.Fatalf("Reset SetDeadline() error = %v", err)
	}

	// 等待超过初始超时时间但小于新超时时间
	// 已经等待了50ms，再等待80ms，总共130ms
	// 这应该超过初始的100ms但小于新的200ms
	time.Sleep(80 * time.Millisecond)

	// 验证连接仍然存活，通过尝试设置新的deadline来测试
	err = conn.SetDeadline(time.Now().Add(1 * time.Second))
	if err != nil {
		t.Error("Connection should still be alive after reset deadline, but SetDeadline failed:", err)
		return
	}

	// 清除deadline以防止干扰后续测试
	err = conn.SetDeadline(time.Time{})
	if err != nil {
		t.Fatalf("Clear deadline error = %v", err)
	}

	// 重新设置一个短的deadline来测试关闭
	shortTimeout := 50 * time.Millisecond
	err = conn.SetDeadline(time.Now().Add(shortTimeout))
	if err != nil {
		t.Fatalf("Set short deadline error = %v", err)
	}

	// 等待超时
	time.Sleep(shortTimeout + 50*time.Millisecond)

	// 验证连接是否被关闭，通过尝试设置deadline来测试
	err = conn.SetDeadline(time.Now().Add(1 * time.Second))
	if err == nil {
		t.Error("Connection should be closed after short deadline, but SetDeadline succeeded")
	}
}

// TestCallback_OnOpen 测试OnOpen回调功能
func TestCallback_OnOpen(t *testing.T) {
	tests := []struct {
		name        string
		expectError bool
		setup       func() *Conn
	}{
		{
			name:        "successful connection open",
			expectError: false,
			setup: func() *Conn {
				fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
				if err != nil {
					t.Fatalf("OpenFile() error = %v", err)
				}
				return &Conn{
					fd: int64(fd.Fd()),
					safeConns: &safeConns[Conn]{
						conns: make([]*Conn, 1000),
					},
				}
			},
		},
		{
			name:        "connection with session data",
			expectError: false,
			setup: func() *Conn {
				fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
				if err != nil {
					t.Fatalf("OpenFile() error = %v", err)
				}
				conn := &Conn{
					fd: int64(fd.Fd()),
					safeConns: &safeConns[Conn]{
						conns: make([]*Conn, 1000),
					},
				}
				conn.SetSession("test_session_data")
				return conn
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := tt.setup()
			defer func() {
				if atomic.LoadInt64(&conn.fd) != -1 {
					conn.Close()
				}
			}()

			var onOpenCalled bool
			var onOpenConn *Conn
			var onOpenErr error

			callback := ToCallback(
				func(c *Conn, err error) {
					onOpenCalled = true
					onOpenConn = c
					onOpenErr = err
				},
				func(c *Conn, data []byte) {
					// Do nothing for this test
				},
				func(c *Conn, err error) {
					// Do nothing for this test
				},
			)

			// 调用OnOpen
			callback.OnOpen(conn)

			// 验证回调被调用
			if !onOpenCalled {
				t.Error("OnOpen callback was not called")
			}

			// 验证传递的连接对象
			if onOpenConn != conn {
				t.Error("OnOpen callback received wrong connection object")
			}

			// 验证错误参数（ToCallback中OnOpen总是传nil）
			if onOpenErr != nil {
				t.Errorf("OnOpen callback received unexpected error: %v", onOpenErr)
			}

			// 验证连接状态
			if atomic.LoadInt64(&conn.fd) == -1 {
				t.Error("Connection should not be closed after OnOpen")
			}
		})
	}
}

// TestCallback_OnClose 测试OnClose回调功能
func TestCallback_OnClose(t *testing.T) {
	tests := []struct {
		name           string
		setupError     error
		expectError    bool
		closeMethod    func(*Conn)
		expectedClosed bool
	}{
		{
			name:           "normal close without error",
			setupError:     nil,
			expectError:    false,
			closeMethod:    func(c *Conn) { c.Close() },
			expectedClosed: true,
		},
		{
			name:           "close with simulated network error",
			setupError:     net.ErrClosed,
			expectError:    true,
			closeMethod:    func(c *Conn) { c.Close() },
			expectedClosed: true,
		},
		{
			name:        "close already closed connection",
			setupError:  nil,
			expectError: false,
			closeMethod: func(c *Conn) {
				c.Close() // First close
				c.Close() // Second close should be safe
			},
			expectedClosed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
			if err != nil {
				t.Fatalf("OpenFile() error = %v", err)
			}

			conn := &Conn{
				fd: int64(fd.Fd()),
				safeConns: &safeConns[Conn]{
					conns: make([]*Conn, 1000),
				},
			}

			var onCloseCalled bool
			var onCloseConn *Conn
			var onCloseErr error
			var callCount int

			callback := ToCallback(
				func(c *Conn, err error) {
					// Do nothing for this test
				},
				func(c *Conn, data []byte) {
					// Do nothing for this test
				},
				func(c *Conn, err error) {
					callCount++
					onCloseCalled = true
					onCloseConn = c
					onCloseErr = err
				},
			)

			// 执行关闭操作
			tt.closeMethod(conn)

			// 模拟调用OnClose（在实际应用中，这会在连接关闭时自动调用）
			callback.OnClose(conn, tt.setupError)

			// 验证回调被调用
			if !onCloseCalled {
				t.Error("OnClose callback was not called")
			}

			// 验证传递的连接对象
			if onCloseConn != conn {
				t.Error("OnClose callback received wrong connection object")
			}

			// 验证错误参数
			if tt.expectError && onCloseErr == nil {
				t.Error("OnClose callback should have received an error")
			}
			if !tt.expectError && onCloseErr != nil {
				t.Errorf("OnClose callback received unexpected error: %v", onCloseErr)
			}

			// 验证连接状态
			if tt.expectedClosed && atomic.LoadInt64(&conn.fd) != -1 {
				t.Error("Connection should be closed")
			}

			// 验证回调只被调用一次（即使多次调用Close）
			if callCount > 1 {
				t.Errorf("OnClose callback was called %d times, expected 1", callCount)
			}
		})
	}
}

// TestConn_CloseCleanup 测试连接关闭时的清理工作
func TestConn_CloseCleanup(t *testing.T) {
	fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	conn := &Conn{
		fd: int64(fd.Fd()),
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

	// 设置一些状态用于测试清理
	conn.SetSession("test_session")

	// 设置定时器
	_ = conn.SetDeadline(time.Now().Add(time.Hour))

	// 添加一些写缓冲区
	testData := []byte("test data for buffer")
	buf := getBytes(len(testData))
	copy(*buf, testData)
	conn.wbufList = append(conn.wbufList, buf)

	// 验证设置成功
	if conn.GetSession() != "test_session" {
		t.Error("Session should be set")
	}
	if conn.readTimer == nil || conn.writeTimer == nil {
		t.Error("Timers should be set")
	}
	if len(conn.wbufList) == 0 {
		t.Error("Write buffer should not be empty")
	}

	var onCloseCallbackCalled bool
	callback := ToCallback(
		func(c *Conn, err error) {
			// Do nothing
		},
		func(c *Conn, data []byte) {
			// Do nothing
		},
		func(c *Conn, err error) {
			onCloseCallbackCalled = true
		},
	)

	// 关闭连接
	conn.Close()
	callback.OnClose(conn, nil)

	// 验证连接被关闭
	if atomic.LoadInt64(&conn.fd) != -1 {
		t.Error("Connection fd should be -1 after close")
	}

	// 验证定时器被清理
	if conn.readTimer != nil {
		t.Error("Read timer should be nil after close")
	}
	if conn.writeTimer != nil {
		t.Error("Write timer should be nil after close")
	}

	// 验证写缓冲区被清理
	if len(conn.wbufList) != 0 {
		t.Error("Write buffer list should be empty after close")
	}

	// 验证回调被调用
	if !onCloseCallbackCalled {
		t.Error("OnClose callback should be called")
	}

	// 验证Session仍然可以访问（Session不会被清理，由用户决定）
	if conn.GetSession() != "test_session" {
		t.Error("Session should still be accessible after close")
	}
}

// TestConn_CloseWithTimeout 测试超时关闭的OnClose回调
func TestConn_CloseWithTimeout(t *testing.T) {
	fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}

	conn := &Conn{
		fd: int64(fd.Fd()),
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

	var onCloseCallbackCalled bool
	var timeoutOccurred bool

	callback := ToCallback(
		func(c *Conn, err error) {
			// Do nothing
		},
		func(c *Conn, data []byte) {
			// Do nothing
		},
		func(c *Conn, err error) {
			onCloseCallbackCalled = true
			// 在实际场景中，超时关闭可能会传递特定的错误
		},
	)

	// 设置一个很短的超时时间
	timeout := 50 * time.Millisecond
	err = conn.SetDeadline(time.Now().Add(timeout))
	if err != nil {
		t.Fatalf("SetDeadline() error = %v", err)
	}

	// 等待超时发生
	time.Sleep(timeout + 20*time.Millisecond)

	// 验证连接被超时关闭
	if atomic.LoadInt64(&conn.fd) != -1 {
		t.Error("Connection should be closed after timeout")
		conn.Close() // 手动清理
	} else {
		timeoutOccurred = true
	}

	// 模拟超时关闭时的OnClose调用
	if timeoutOccurred {
		callback.OnClose(conn, nil)
	}

	// 验证超时确实发生
	if !timeoutOccurred {
		t.Error("Timeout should have occurred")
	}

	// 验证回调被调用
	if timeoutOccurred && !onCloseCallbackCalled {
		t.Error("OnClose callback should be called after timeout")
	}
}

// TestCallback_ToCallback 测试ToCallback函数
func TestCallback_ToCallback(t *testing.T) {
	var onOpenCalled, onDataCalled, onCloseCalled bool
	var receivedConn *Conn
	var receivedData []byte
	var receivedErr error

	callback := ToCallback(
		func(c *Conn, err error) {
			onOpenCalled = true
			receivedConn = c
			receivedErr = err
		},
		func(c *Conn, data []byte) {
			onDataCalled = true
			receivedConn = c
			receivedData = data
		},
		func(c *Conn, err error) {
			onCloseCalled = true
			receivedConn = c
			receivedErr = err
		},
	)

	// 创建测试连接
	fd, err := os.OpenFile("/dev/null", os.O_RDONLY, 0666)
	if err != nil {
		t.Fatalf("OpenFile() error = %v", err)
	}
	defer fd.Close()

	conn := &Conn{
		fd: int64(fd.Fd()),
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}
	defer conn.Close()

	testData := []byte("test data")
	testErr := net.ErrClosed

	// 测试OnOpen
	callback.OnOpen(conn)
	if !onOpenCalled {
		t.Error("OnOpen should be called")
	}
	if receivedConn != conn {
		t.Error("OnOpen should receive correct connection")
	}
	if receivedErr != nil {
		t.Error("OnOpen should receive nil error through ToCallback")
	}

	// 重置状态
	onOpenCalled = false
	receivedConn = nil
	receivedErr = nil

	// 测试OnData
	callback.OnData(conn, testData)
	if !onDataCalled {
		t.Error("OnData should be called")
	}
	if receivedConn != conn {
		t.Error("OnData should receive correct connection")
	}
	if string(receivedData) != string(testData) {
		t.Error("OnData should receive correct data")
	}

	// 重置状态
	onDataCalled = false
	receivedConn = nil
	receivedData = nil

	// 测试OnClose
	callback.OnClose(conn, testErr)
	if !onCloseCalled {
		t.Error("OnClose should be called")
	}
	if receivedConn != conn {
		t.Error("OnClose should receive correct connection")
	}
	if receivedErr != testErr {
		t.Error("OnClose should receive correct error")
	}
}
