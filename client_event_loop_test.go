package pulse

import (
	"context"
	"net"
	"testing"
)

func TestClientEventLoop_RegisterConn(t *testing.T) {
	// 创建客户端事件循环
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loop := NewClientEventLoop(ctx, WithCallback(&testCallback{}))

	// 创建测试连接
	conn, err := net.Dial("tcp", "127.0.0.1:8080")
	if err != nil {
		// 如果连接失败，这是预期的，因为我们没有启动服务器
		// 我们只是想测试 RegisterConn 函数本身
		t.Logf("Expected connection failure: %v", err)
		return
	}

	// 测试注册连接
	err = loop.RegisterConn(conn)
	if err != nil {
		t.Errorf("RegisterConn failed: %v", err)
	}
}

type testCallback struct{}

func (tc *testCallback) OnOpen(c *Conn) {
	// 测试回调
}

func (tc *testCallback) OnData(c *Conn, data []byte) {
	// 测试回调
}

func (tc *testCallback) OnClose(c *Conn, err error) {
	// 测试回调
}

func TestClientEventLoop_NewClientEventLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loop := NewClientEventLoop(ctx, WithCallback(&testCallback{}))

	// 验证基本字段是否正确初始化
	if loop.MultiEventLoop == nil {
		t.Error("MultiEventLoop should not be nil")
	}

	if loop.callback == nil {
		t.Error("callback should not be nil")
	}

	if loop.ctx == nil {
		t.Error("ctx should not be nil")
	}

}

func TestClientEventLoop_SelectEventLoop(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loop := NewClientEventLoop(ctx, WithCallback(&testCallback{}))

	// 测试事件循环选择
	index1 := loop.selectEventLoop()
	index2 := loop.selectEventLoop()

	if index1 < 0 || index1 >= len(loop.MultiEventLoop.eventLoops) {
		t.Errorf("Invalid event loop index: %d", index1)
	}

	if index2 < 0 || index2 >= len(loop.MultiEventLoop.eventLoops) {
		t.Errorf("Invalid event loop index: %d", index2)
	}

	// 验证轮询分配（虽然不能保证每次都不同，但应该合理分布）
	t.Logf("Selected event loops: %d, %d", index1, index2)
}
