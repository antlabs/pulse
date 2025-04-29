package main

import (
	"context"
	"fmt"
	"log/slog"

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

func main() {

	go func() {
		http.ListenAndServe(":7777", nil)
	}()

	el, err := pulse.NewMultiEventLoop(
		context.Background(),
		pulse.WithCallback(&handler{}),
		pulse.WithLogLevel[[]byte](slog.LevelDebug),
	)
	if err != nil {
		panic(err.Error())
	}

	slog.Info("Server started on :8080")
	el.ListenAndServe(":8080")
}
