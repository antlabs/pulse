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

func BenchmarkSafeConns_Get(b *testing.B) {
	var safeConns safeConns[testConn]
	safeConns.init(1000)

	// 预先添加一些连接
	for i := 0; i < 100; i++ {
		conn := &testConn{id: i}
		safeConns.Add(i, conn)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		fd := 50 // 使用中间的一个fd进行测试
		for pb.Next() {
			_ = safeConns.Get(fd)
		}
	})
}

func BenchmarkSafeConns_Get_Sequential(b *testing.B) {
	var safeConns safeConns[testConn]
	safeConns.init(1000)

	// 预先添加一些连接
	for i := 0; i < 100; i++ {
		conn := &testConn{id: i}
		safeConns.Add(i, conn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd := i % 100 // 循环访问不同的fd
		_ = safeConns.Get(fd)
	}
}

func BenchmarkSafeConns_Get_NotFound(b *testing.B) {
	var safeConns safeConns[testConn]
	safeConns.init(1000)

	// 预先添加一些连接
	for i := 0; i < 100; i++ {
		conn := &testConn{id: i}
		safeConns.Add(i, conn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 测试访问不存在的fd
		_ = safeConns.Get(999)
	}
}

func BenchmarkSafeConns_Get_OutOfRange(b *testing.B) {
	var safeConns safeConns[testConn]
	safeConns.init(100)

	// 预先添加一些连接
	for i := 0; i < 50; i++ {
		conn := &testConn{id: i}
		safeConns.Add(i, conn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 测试访问超出范围的fd
		_ = safeConns.Get(1000)
	}
}

// 不使用原子变量的简化版 Get 方法，仅用于性能对比
func (s *safeConns[T]) GetNonAtomic(fd int) *T {
	if fd == -1 {
		return nil
	}

	if fd >= len(s.conns) {
		return nil
	}

	return s.conns[fd]
}

func BenchmarkSafeConns_Get_NonAtomic(b *testing.B) {
	var safeConns safeConns[testConn]
	safeConns.init(1000)

	// 预先添加一些连接
	for i := 0; i < 100; i++ {
		conn := &testConn{id: i}
		safeConns.Add(i, conn)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		fd := 50 // 使用中间的一个fd进行测试
		for pb.Next() {
			_ = safeConns.GetNonAtomic(fd)
		}
	})
}

func BenchmarkSafeConns_Get_NonAtomic_Sequential(b *testing.B) {
	var safeConns safeConns[testConn]
	safeConns.init(1000)

	// 预先添加一些连接
	for i := 0; i < 100; i++ {
		conn := &testConn{id: i}
		safeConns.Add(i, conn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fd := i % 100 // 循环访问不同的fd
		_ = safeConns.GetNonAtomic(fd)
	}
}

func BenchmarkSafeConns_Get_NonAtomic_NotFound(b *testing.B) {
	var safeConns safeConns[testConn]
	safeConns.init(1000)

	// 预先添加一些连接
	for i := 0; i < 100; i++ {
		conn := &testConn{id: i}
		safeConns.Add(i, conn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 测试访问不存在的fd
		_ = safeConns.GetNonAtomic(999)
	}
}

func BenchmarkSafeConns_Get_NonAtomic_OutOfRange(b *testing.B) {
	var safeConns safeConns[testConn]
	safeConns.init(100)

	// 预先添加一些连接
	for i := 0; i < 50; i++ {
		conn := &testConn{id: i}
		safeConns.Add(i, conn)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 测试访问超出范围的fd
		_ = safeConns.GetNonAtomic(1000)
	}
}
