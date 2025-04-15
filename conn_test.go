package pulse

import (
	"fmt"
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
