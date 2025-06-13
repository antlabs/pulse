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

	options.taskType = TaskTypeInEventLoop
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
				if atomic.LoadInt64(&conn.fd) != -1 {
					t.Error("Connection should be closed for past deadline")
				}
				return
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
				if atomic.LoadInt64(&conn.fd) != -1 {
					t.Error("Connection should be closed for past deadline")
				}
				return
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
				if atomic.LoadInt64(&conn.fd) != -1 {
					t.Error("Connection should be closed for past deadline")
				}
				return
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

	// 设置初始超时
	initialTimeout := 300 * time.Millisecond
	err = conn.SetDeadline(time.Now().Add(initialTimeout))
	if err != nil {
		t.Fatalf("SetDeadline() error = %v", err)
	}

	// 等待一半时间
	time.Sleep(initialTimeout / 2) // 等待150ms

	// 重置超时时间为更长的时间
	newTimeout := 500 * time.Millisecond
	err = conn.SetDeadline(time.Now().Add(newTimeout))
	if err != nil {
		t.Fatalf("Reset SetDeadline() error = %v", err)
	}

	// 等待超过初始超时时间但小于新超时时间
	// 已经等待了150ms，再等待200ms，总共350ms，超过初始300ms但小于新的500ms
	time.Sleep(200 * time.Millisecond)

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
