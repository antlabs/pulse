package pulse

import (
	"fmt"
	"net"
	"testing"
)

func Test_OnData(t *testing.T) {
	c := &Conn{fd: 0}
	options := &Options[[]byte]{
		callback: ToCallback[[]byte](
			func(c *Conn, err error) {
				fmt.Println("OnOpen:", err)
			},
			func(c *Conn, data []byte) {
				fmt.Println("OnData:", data)
			},
			func(c *Conn, err error) {
				fmt.Println("OnClose:", err)
			},
		),
	}
	handleData[[]byte](c, options, []byte("hello"))
}

func Test_Listen(t *testing.T) {
	fmt.Println(net.Listen("tcp", "127.0.0.1:8080"))
	fmt.Println(net.Listen("tcp", "127.0.0.1:8080"))
}
