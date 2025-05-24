//go:build unix || darwin || linux

package pulse

import "syscall"

var maxFd int

func init() {
	var limit syscall.Rlimit
	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &limit)
	if err != nil {
		panic(err)
	}
	maxFd = int(limit.Cur)
}
