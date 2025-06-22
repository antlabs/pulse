package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/antlabs/pulse"
)

type handler struct {
	name string
}

func (h *handler) OnOpen(c *pulse.Conn) {
	slog.Info("connection opened", "name", h.name)
}

func (h *handler) OnData(c *pulse.Conn, data []byte) {
	slog.Info("received data", "name", h.name, "size", len(data))

	// 模拟回显大量数据，可能触发流量背压
	largeResponse := make([]byte, 10240) // 10KB 响应
	copy(largeResponse, data)

	n, err := c.Write(largeResponse)
	if err != nil {
		slog.Error("failed to write response", "error", err)
		return
	}
	slog.Info("sent response", "bytes", n)
}

func (h *handler) OnClose(c *pulse.Conn, err error) {
	if err != nil {
		slog.Info("connection closed with error", "name", h.name, "error", err)
	} else {
		slog.Info("connection closed normally", "name", h.name)
	}
}

func main() {
	// 创建启用流量背压删除读事件机制的服务器
	server, err := pulse.NewMultiEventLoop(
		context.Background(),
		pulse.WithCallback(&handler{name: "flow-backpressure-server"}),
		pulse.WithTaskType(pulse.TaskTypeInEventLoop),
		pulse.WithTriggerType(pulse.TriggerTypeEdge),
		pulse.WithLogLevel(slog.LevelInfo),
		pulse.WithFlowBackPressureRemoveRead(true), // 启用流量背压删除读事件机制
	)
	if err != nil {
		panic(err)
	}

	go func() {
		fmt.Println("Server starting on :8080 with flow backpressure (remove read events) enabled")
		if err := server.ListenAndServe(":8080"); err != nil {
			panic(err)
		}
	}()

	// 保持服务器运行
	time.Sleep(time.Hour)
}
