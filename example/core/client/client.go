package main

import "github.com/antlabs/pulse/core"

func main() {
	as, err := core.Create()
	if err != nil {
		return
	}
	as.AddRead()
}
