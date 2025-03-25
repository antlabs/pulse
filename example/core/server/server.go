package main

import (
	"time"

	"github.com/antlabs/pulse/core"
)

func main() {

	as, err := core.Create()
	if err != nil {
		return
	}
	// 使用示例文件描述符 0 (标准输入)
	go as.Accept("tcp", "127.0.0.1:8080")

	as.Poll(time.Second*10, func(fd int, state core.State, err error) {
		if err != nil {
			return
		}
		if state&core.READ != 0 {

		}

		if state&core.WRITE != 0 {

		}
	})
}
