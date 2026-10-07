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
	"unsafe"
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
// 分两段: 先非阻塞探一次, 真没事件才进 Read 去 park。
//
// 探测必须放在 Read 外面: Read 不是免费的——它要路过
// entersyscall/exitsyscall, 还要在 runtime 的 netpoller 上登记, 唤醒时
// 再摘下来。忙碌的服务器上大多数轮次都有事件, 这种轮次里进一次 Read 再
// 让回调返回 1 把它拉出来, 等于为 park 付了钱却没 park。提到外面之后,
// 有事件的轮次和阻塞式一样只是一次 epoll_wait(0), 只有真空转的轮次才
// 去 park——而那正是需要 park 的地方, 把 P 让给解析 goroutine。
func (w *epollWaiter) wait(epfd int, events []syscall.EpollEvent) (int, error) {
	if w == nil {
		return epollWaitRaw(epfd, events, -1)
	}
	// 第一段: 有事件就地取走, 不进 runtime。
	n, err := epollWaitRaw(epfd, events, 0)
	if n != 0 || (err != nil && err != syscall.EINTR) {
		return n, err
	}
	// 第二段: 确实没有, park 着等。
	// 事件在这两段之间到达不会丢: epoll 是状态, Read 的回调里再探一次就
	// 能看到; 万一线程已经睡下, epoll fd 仍可读, runtime 会立刻唤醒。
	var err2 error
	readErr := w.conn.Read(func(uintptr) bool {
		n, err2 = epollWaitRaw(epfd, events, 0)
		return n != 0 || (err2 != nil && err2 != syscall.EINTR)
	})
	if readErr == nil {
		return n, err2
	}
	// runtime 轮询不了这个 fd, 退回阻塞式。
	return epollWaitRaw(epfd, events, -1)
}

func (w *epollWaiter) close() {
	if w != nil {
		w.file.Close()
	}
}

// epollWaitRaw 是 RawSyscall6 版的 epoll_wait。
//
// syscall.EpollWait 内部是 Syscall6, 那比 RawSyscall6 多一对
// entersyscall/exitsyscall(runtime 要在进出时处理 P 的绑定和抢占检查)。
// epoll_wait 带 0 超时时不会阻塞, 不需要把 P 交出去, 那对调用纯属白付。
//
// 实测(8 P 核 1KB echo): 事件循环等待那条路径占 6.6% CPU, 换掉之后
// 省下的是 entersyscall/exitsyscall 那部分。
func epollWaitRaw(epfd int, events []syscall.EpollEvent, msec int) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}
	r, _, errno := syscall.RawSyscall6(syscall.SYS_EPOLL_WAIT,
		uintptr(epfd),
		uintptr(unsafe.Pointer(&events[0])),
		uintptr(len(events)),
		uintptr(msec), 0, 0)
	if errno != 0 {
		return 0, errno
	}
	return int(r), nil
}
