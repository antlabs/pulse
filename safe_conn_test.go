package pulse

import (
	"testing"
)

type testConn struct {
	id int
}

func TestSafeConns_AddAndGet(t *testing.T) {
	var safeConns safeConns[testConn]
	safeConns.init(100)

	// 测试添加和获取连接
	conn := &testConn{id: 1}
	fd := 5

	// 添加连接
	safeConns.Add(fd, conn)

	// 获取连接
	got := safeConns.Get(fd)
	if got == nil {
		t.Errorf("Get(%d) = nil, want %v", fd, conn)
	}
	if got.id != conn.id {
		t.Errorf("Get(%d).id = %d, want %d", fd, got.id, conn.id)
	}

	// 测试获取不存在的连接
	notExistFd := 999
	if got := safeConns.Get(notExistFd); got != nil {
		t.Errorf("Get(%d) = %v, want nil", notExistFd, got)
	}

	// 测试删除连接
	safeConns.Del(fd)
	if got := safeConns.Get(fd); got != nil {
		t.Errorf("Get(%d) after Del = %v, want nil", fd, got)
	}
}

func TestSafeConns_Concurrent(t *testing.T) {
	var safeConns safeConns[testConn]
	safeConns.init(1000)

	// 并发测试
	t.Run("concurrent add and get", func(t *testing.T) {
		const goroutines = 10
		done := make(chan bool)

		for i := 0; i < goroutines; i++ {
			go func(id int) {
				fd := id + 1
				conn := &testConn{id: id}

				// 添加连接
				safeConns.Add(fd, conn)

				// 获取连接
				got := safeConns.Get(fd)
				if got == nil {
					t.Errorf("Get(%d) = nil, want %v", fd, conn)
				}
				if got.id != conn.id {
					t.Errorf("Get(%d).id = %d, want %d", fd, got.id, conn.id)
				}

				done <- true
			}(i)
		}

		// 等待所有 goroutine 完成
		for i := 0; i < goroutines; i++ {
			<-done
		}
	})
}
