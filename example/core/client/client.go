package main

import (
	"fmt"
	"reflect"
	"syscall"
	"time"

	"github.com/antlabs/pulse/core"
)

func printSize(iface interface{}) bool {
	val := reflect.ValueOf(iface)
	return val.Type() == reflect.TypeOf(struct{}{})
}

type handler struct{}

func main() {

	fmt.Println(printSize(handler{}))
	as, err := core.Create(core.TriggerTypeLevel)
	if err != nil {
		return
	}
	// 使用示例文件描述符 0 (标准输入)
	fd, err := core.Dial("tcp", "127.0.0.1:8080", as)
	if err != nil {
		return
	}

	syscall.Write(fd, []byte("hello"))
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
