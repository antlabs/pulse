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
// +build linux

package core

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"syscall"
	"time"
	"unsafe"
)

const (
	// 垂直触发
	// 来自man 手册
	// When  used as an edge-triggered interface, for performance reasons,
	// it is possible to add the file descriptor inside the epoll interface (EPOLL_CTL_ADD) once by specifying (EPOLLIN|EPOLLOUT).
	// This allows you to avoid con‐
	// tinuously switching between EPOLLIN and EPOLLOUT calling epoll_ctl(2) with EPOLL_CTL_MOD.
	etAddRead   = int(syscall.EPOLLERR | syscall.EPOLLHUP | syscall.EPOLLRDHUP | syscall.EPOLLPRI | syscall.EPOLLIN | syscall.EPOLLOUT | -syscall.EPOLLET)
	etAddWrite  = int(0)
	etDelRead   = int(syscall.EPOLLERR | syscall.EPOLLHUP | syscall.EPOLLRDHUP | syscall.EPOLLPRI | syscall.EPOLLOUT | -syscall.EPOLLET)
	etDelWrite  = int(syscall.EPOLLERR | syscall.EPOLLHUP | syscall.EPOLLRDHUP | syscall.EPOLLPRI | syscall.EPOLLOUT | -syscall.EPOLLET)
	etResetRead = int(etAddRead)

	// 水平触发
	ltAddRead   = int(syscall.EPOLLIN | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR | syscall.EPOLLPRI)
	ltAddWrite  = int(ltAddRead | syscall.EPOLLOUT)
	ltDelRead   = int(syscall.EPOLLOUT | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR | syscall.EPOLLPRI)
	ltDelWrite  = int(ltAddRead)
	ltResetRead = int(ltAddRead)

	// 写事件
	processWrite = uint32(syscall.EPOLLOUT)
	// 读事件
	processRead = uint32(syscall.EPOLLIN | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR)
)

var _ PollingApi = (*eventPollState)(nil)

type eventPollState struct {
	epfd   int
	events []syscall.EpollEvent

	et      bool
	rev     int
	wev     int
	drEv    int // delete read event
	dwEv    int // delete write event
	resetEv int

	// waiter 让等待 park 在 runtime 的 poller 上而不是阻塞在 syscall 里,
	// 见 waiter_linux.go。别的平台为 nil, 走阻塞式。
	waiter *epollWaiter
	// waitParked 关掉 waiter, 退回 syscall.EpollWait。
	waitParked bool
}

func getReadWriteDeleteReset(et bool) (int, int, int, int, int) {
	if et {
		return etAddRead, etAddWrite, etDelRead, etDelWrite, etResetRead
	}

	return ltAddRead, ltAddWrite, etDelRead, ltDelWrite, ltResetRead
}

// 创建epoll handler
func Create(triggerType TriggerType) (la PollingApi, err error) {
	var e eventPollState
	e.epfd, err = syscall.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	slog.Info("create epoll", "triggerType", triggerType)
	e.waiter = newEpollWaiter(e.epfd)
	e.events = make([]syscall.EpollEvent, 1024)
	e.rev, e.wev, e.drEv, e.dwEv, e.resetEv = getReadWriteDeleteReset(triggerType == TriggerTypeEdge)
	return &e, nil
}

// 释放
func (e *eventPollState) Free() {
	// waiter 持有 epoll fd 的一份 dup, 先放掉它再关自己的。
	e.waiter.close()
	if err := syscall.Close(e.epfd); err != nil {
		// Log the error but don't panic as this is a cleanup function
		slog.Warn("failed to close epoll fd", "error", err)
	}
}

// 新加读事件
func (e *eventPollState) AddRead(fd int) error {
	if e.rev > 0 && fd >= 0 {
		return epollCtlRaw(e.epfd, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(e.rev),
		})
	}
	return nil
}

// 新加写事件
func (e *eventPollState) AddWrite(fd int) error {
	if e.wev > 0 && fd >= 0 {
		return epollCtlRaw(e.epfd, syscall.EPOLL_CTL_MOD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(e.wev),
		})
	}

	return nil
}

func (e *eventPollState) ResetRead(fd int) error {
	if e.resetEv > 0 && fd >= 0 {
		return epollCtlRaw(e.epfd, syscall.EPOLL_CTL_MOD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(e.resetEv),
		})
	}
	return nil
}

// 删除写事件
func (e *eventPollState) DelWrite(fd int) error {
	if e.dwEv > 0 {
		return epollCtlRaw(e.epfd, syscall.EPOLL_CTL_MOD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(e.dwEv),
		})
	}
	return nil
}

// 删除读事件
func (e *eventPollState) DelRead(fd int) error {
	if fd > 0 {
		// 移除读事件，只保留写事件
		return epollCtlRaw(e.epfd, syscall.EPOLL_CTL_MOD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(syscall.EPOLLOUT),
		})
	}
	return nil
}

// 删除事件
func (e *eventPollState) Del(fd int) error {
	return epollCtlRaw(e.epfd, syscall.EPOLL_CTL_DEL, fd, &syscall.EpollEvent{Fd: int32(fd)})
}

// noPark 从 PULSE_NO_PARK 读一次, 关掉 park, 见 Poll。
var noPark = os.Getenv("PULSE_NO_PARK") != ""

// 事件循环
func (e *eventPollState) Poll(tv time.Duration, cb func(fd int, state State, err error)) (numEvents int, err error) {
	msec := -1
	if tv > 0 {
		msec = int(tv) / int(time.Millisecond)
	}

	// park 在 runtime 的 poller 上等, 还是阻塞在 epoll_wait 里。
	//
	// 阻塞在 syscall 里会一直占着 P, 直到 runtime 的监控线程把 P 抢走,
	// 而排在这个 P 上的 goroutine 这段时间全在等; park 则是一瞬间就把
	// P 交出去, fd 可读了 runtime 再唤醒它。见 waiter_linux.go。
	//
	// 长超时也走 park: 调用方给的那点超时是兜底, 它要的是"fd 可读时
	// 立刻回来", runtime 的 poller 正好是这个语义。短超时(下面这个
	// 门槛以内)才留给阻塞式, 那种调用是真的要按时返回。
	//
	// PULSE_NO_PARK 关掉它, 退回阻塞 epoll_wait, 用于对照两者。
	const parkAbove = time.Second
	usePark := e.waiter != nil && !e.waitParked && !noPark &&
		(tv <= 0 || tv >= parkAbove)
	if usePark {
		numEvents, err = e.waiter.wait(e.epfd, e.events)
	} else {
		// 非 park 那条也用 RawSyscall6: syscall.EpollWait 走的是 Syscall6,
		// 多一对 entersyscall/exitsyscall。带超时的 epoll_wait 确实会睡,
		// 但那段时间本来就不占 CPU 记账, 而进出两次 runtime 调用是实打实
		// 的 (实测事件循环等待路径占 6.6% CPU)。
		numEvents, err = epollWaitRaw(e.epfd, e.events, msec)
	}
	if err != nil {
		if errors.Is(err, syscall.EINTR) {
			return 0, nil
		}
		return 0, err
	}

	for i := 0; i < numEvents; i++ {
		ev := &e.events[i]
		fd := ev.Fd

		// unix.EPOLLRDHUP是关闭事件，遇到直接关闭
		if ev.Events&(syscall.EPOLLERR|syscall.EPOLLHUP|syscall.EPOLLRDHUP) > 0 {
			cb(int(fd), WRITE|READ, io.EOF)
			continue
		}
		var state State

		if ev.Events&processRead > 0 {
			state |= READ
		}
		if ev.Events&processWrite > 0 {
			state |= WRITE
		}

		cb(int(fd), state, nil)

	}

	return numEvents, nil
}

func (e *eventPollState) Name() string {
	return "epoll"
}

// epollCtlRaw 是 RawSyscall6 版的 epoll_ctl。
//
// syscall.EpollCtl 走 Syscall6(带 entersyscall/exitsyscall), 而 epoll_ctl
// 不会阻塞——不需要把那对调用付在建连/改事件这种每次连接都要做的操作上。
//
// 只有 epoll_wait 是可能阻塞的, 那个的快速路径改了(见 waiter_linux.go),
// 阻塞等待仍走 runtime 的 park。
func epollCtlRaw(epfd, op, fd int, ev *syscall.EpollEvent) error {
	_, _, errno := syscall.RawSyscall6(syscall.SYS_EPOLL_CTL,
		uintptr(epfd), uintptr(op), uintptr(fd),
		uintptr(unsafe.Pointer(ev)), 0, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
