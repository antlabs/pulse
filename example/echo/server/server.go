package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"net/http"
	_ "net/http/pprof"

	"github.com/antlabs/pulse"
)

// 必须是空结构体
type handler struct{}

func (h *handler) OnOpen(c *pulse.Conn, err error) {
	if err != nil {
		fmt.Println("OnOpen error:", err)
		return
	}
	fmt.Println("OnOpen success")
}

func (h *handler) OnData(c *pulse.Conn, data []byte) {
	// fmt.Println("OnData:", string(data))
	// fmt.Printf("OnData: %d, %p\n", len(data), c)

	// fd, err := os.OpenFile("test.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	// if err != nil {
	// 	panic(err.Error())
	// }
	// defer fd.Close()

	// fd.Write(data)

	c.Write(data)
}

func (h *handler) OnClose(c *pulse.Conn, err error) {
	if err != nil {
		fmt.Println("OnClose error:", err)
		return
	}
	fmt.Println("OnClose success")
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			fmt.Println("Read error:", err)
			return
		}
		_, err = conn.Write(buf[:n])
		if err != nil {
			fmt.Println("Write error:", err)
			return
		}
	}
}

// Start standard library echo server
func startStandardEchoServer() {
	listener, err := net.Listen("tcp", ":8081")
	if err != nil {
		panic(err.Error())
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Accept error:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func main() {

	// Start standard library echo server
	go startStandardEchoServer()

	go func() {
		http.ListenAndServe(":8082", nil)
	}()

	// Start pulse echo server
	el, err := pulse.NewMultiEventLoop(
		context.Background(),
		pulse.WithCallback(&handler{}),
		pulse.WithLogLevel(slog.LevelError),
		pulse.WithTaskType(pulse.TaskTypeInEventLoop),
		pulse.WithTriggerType(pulse.TriggerTypeEdge),
		pulse.WithEventLoopReadBufferSize(1024*4))

	if err != nil {
		panic(err.Error())
	}

	slog.Info("Pulse echo server started on :8080")
	el.ListenAndServe(":8080")
}
