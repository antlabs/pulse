package main

import (
	"fmt"

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
	fmt.Println("OnData:", string(data))
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

	el, err := pulse.NewMultiEventLoop(
		pulse.WithCallback(&handler{}),
	)
	if err != nil {
		panic(err.Error())
	}

	el.ListenAndServe(":8080")
}
