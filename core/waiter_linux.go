// Copyright 2023-2024 antlabs. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build linux

package core

import (
	"os"
	"syscall"
)

// epollWaiter 把 epoll 的等待从 "阻塞在系统调用里" 改成 "park 在 runtime
// 的网络轮询器上"。
//
// 一个 goroutine 阻塞在系统调用里会一直占着它的 P, runtime 要等 sysmon
// 一个 tick(几十微秒到十毫秒)之后才把 P 抢走, 而排在那个 P 上的 goroutine
// 这段时间全在等。事件循环是"等一下就醒"的循环——每来一个连接、每收到
// 一批事件就醒一次——每次都攥着 P 不放一会, 别的 goroutine 就跟着等。
//
// park 在 runtime 的 poller 上则是一瞬间交出 P, runtime 通知它 epoll fd
// 可读了才唤醒。
//
// 这是 fib 的 poller_epoll.go 里做的事, 它的注释记着实测: 60k 连接的
// TLS 握手突发, 三个 CPU 上阻塞式等待有 10%~15% 的空闲, park 之后降到
// 0~2%, HTTP/1 建连快 19%, echo 不变。
//
// runtime 轮询的是复制出来的 fd, 引擎自己那个照旧用、照旧关。
type epollWaiter struct {
	file *os.File
	conn syscall.RawConn
}

func newEpollWaiter(epfd int) *epollWaiter {
	fd, err := syscall.Dup(epfd)
	if err != nil {
		return nil
	}
	syscall.CloseOnExec(fd)
	// runtime 只轮询非阻塞的 fd。这个标志和引擎自己那个 fd 共享,
	// 而 epoll_wait 不看它。
	if err := syscall.SetNonblock(fd, true); err != nil {
		syscall.Close(fd)
		return nil
	}
	file := os.NewFile(uintptr(fd), "pulse-epoll")
	conn, err := file.SyscallConn()
	if err != nil {
		file.Close()
		return nil
	}
	return &epollWaiter{file: file, conn: conn}
}

// wait 返回就绪的事件数, 没有就等着。
//
// Read 里先自己找一遍事件, runtime 报告可读时再找一遍, 所以有事件排着的
// 时候和阻塞式一样只调一次 epoll_wait。
func (w *epollWaiter) wait(epfd int, events []syscall.EpollEvent) (int, error) {
	if w == nil {
		return syscall.EpollWait(epfd, events, -1)
	}
	var n int
	var err error
	readErr := w.conn.Read(func(uintptr) bool {
		n, err = syscall.EpollWait(epfd, events, 0)
		return n != 0 || (err != nil && err != syscall.EINTR)
	})
	if readErr == nil {
		return n, err
	}
	// runtime 轮询不了这个 fd, 退回阻塞式。
	return syscall.EpollWait(epfd, events, -1)
}

func (w *epollWaiter) close() {
	if w != nil {
		w.file.Close()
	}
}
