package pulse

import (
	"net"
	"sync/atomic"
	"testing"
	"time"
)

func Test_OnData(t *testing.T) {
	c := &Conn{fd: 0}
	var receivedData []byte
	var receivedErr error
	var closeErr error

	options := &Options[[]byte]{
		callback: ToCallback[[]byte](
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

	testData := []byte("hello")
	handleData[[]byte](c, options, testData)

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
	// 创建一个测试用的Conn
	conn := &Conn{
		fd: 1,
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

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
			err := conn.SetDeadline(tt.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetDeadline() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 验证读写定时器都被正确设置
			if !tt.time.IsZero() {
				if conn.readTimer == nil {
					t.Error("readTimer should not be nil")
				}
				if conn.writeTimer == nil {
					t.Error("writeTimer should not be nil")
				}
			} else {
				if conn.readTimer != nil {
					t.Error("readTimer should be nil")
				}
				if conn.writeTimer != nil {
					t.Error("writeTimer should be nil")
				}
			}
		})
	}

	// 测试连接关闭的情况
	conn.close()
	err := conn.SetDeadline(time.Now().Add(time.Second))
	if err == nil {
		t.Error("SetDeadline() should return error when connection is closed")
	}
}

func TestConn_SetReadDeadline(t *testing.T) {
	conn := &Conn{
		fd: 1,
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

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
			err := conn.SetReadDeadline(tt.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetReadDeadline() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 验证读定时器被正确设置
			if !tt.time.IsZero() {
				if conn.readTimer == nil {
					t.Error("readTimer should not be nil")
				}
			} else {
				if conn.readTimer != nil {
					t.Error("readTimer should be nil")
				}
			}
		})
	}

	// 测试连接关闭的情况
	conn.close()
	err := conn.SetReadDeadline(time.Now().Add(time.Second))
	if err == nil {
		t.Error("SetReadDeadline() should return error when connection is closed")
	}
}

func TestConn_SetWriteDeadline(t *testing.T) {
	conn := &Conn{
		fd: 1,
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

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
			err := conn.SetWriteDeadline(tt.time)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetWriteDeadline() error = %v, wantErr %v", err, tt.wantErr)
			}

			// 验证写定时器被正确设置
			if !tt.time.IsZero() {
				if conn.writeTimer == nil {
					t.Error("writeTimer should not be nil")
				}
			} else {
				if conn.writeTimer != nil {
					t.Error("writeTimer should be nil")
				}
			}
		})
	}

	// 测试连接关闭的情况
	conn.close()
	err := conn.SetWriteDeadline(time.Now().Add(time.Second))
	if err == nil {
		t.Error("SetWriteDeadline() should return error when connection is closed")
	}
}

func TestConn_DeadlineTimeout(t *testing.T) {
	conn := &Conn{
		fd: 1,
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

	// 设置一个很短的超时时间
	timeout := 100 * time.Millisecond
	err := conn.SetDeadline(time.Now().Add(timeout))
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
	conn := &Conn{
		fd: 1,
		safeConns: &safeConns[Conn]{
			conns: make([]*Conn, 1000),
		},
	}

	// 设置初始超时
	initialTimeout := 200 * time.Millisecond
	err := conn.SetDeadline(time.Now().Add(initialTimeout))
	if err != nil {
		t.Fatalf("SetDeadline() error = %v", err)
	}

	// 等待一半时间
	time.Sleep(initialTimeout / 2)

	// 重置超时时间
	newTimeout := 200 * time.Millisecond
	err = conn.SetDeadline(time.Now().Add(newTimeout))
	if err != nil {
		t.Fatalf("Reset SetDeadline() error = %v", err)
	}

	// 等待超过初始超时时间但小于新超时时间
	time.Sleep(initialTimeout + 50*time.Millisecond)

	// 验证连接仍然存活
	if atomic.LoadInt64(&conn.fd) == -1 {
		t.Error("Connection should still be alive after reset deadline")
	}

	// 等待新超时时间
	time.Sleep(newTimeout + 50*time.Millisecond)

	// 验证连接是否被关闭
	if atomic.LoadInt64(&conn.fd) != -1 {
		t.Error("Connection should be closed after new deadline")
	}
}
