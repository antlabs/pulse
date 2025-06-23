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
	"syscall"
	"time"
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
	e.events = make([]syscall.EpollEvent, 1024)
	e.rev, e.wev, e.drEv, e.dwEv, e.resetEv = getReadWriteDeleteReset(triggerType == TriggerTypeEdge)
	return &e, nil
}

// 释放
func (e *eventPollState) Free() {
	if err := syscall.Close(e.epfd); err != nil {
		// Log the error but don't panic as this is a cleanup function
		slog.Warn("failed to close epoll fd", "error", err)
	}
}

// 新加读事件
func (e *eventPollState) AddRead(fd int) error {
	if e.rev > 0 && fd >= 0 {
		return syscall.EpollCtl(e.epfd, syscall.EPOLL_CTL_ADD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(e.rev),
		})
	}
	return nil
}

// 新加写事件
func (e *eventPollState) AddWrite(fd int) error {
	if e.wev > 0 && fd >= 0 {
		return syscall.EpollCtl(e.epfd, syscall.EPOLL_CTL_MOD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(e.wev),
		})
	}

	return nil
}

func (e *eventPollState) ResetRead(fd int) error {
	if e.resetEv > 0 && fd >= 0 {
		return syscall.EpollCtl(e.epfd, syscall.EPOLL_CTL_MOD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(e.resetEv),
		})
	}
	return nil
}

// 删除写事件
func (e *eventPollState) DelWrite(fd int) error {
	if e.dwEv > 0 {
		return syscall.EpollCtl(e.epfd, syscall.EPOLL_CTL_MOD, fd, &syscall.EpollEvent{
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
		return syscall.EpollCtl(e.epfd, syscall.EPOLL_CTL_MOD, fd, &syscall.EpollEvent{
			Fd:     int32(fd),
			Events: uint32(syscall.EPOLLOUT),
		})
	}
	return nil
}

// 删除事件
func (e *eventPollState) Del(fd int) error {
	return syscall.EpollCtl(e.epfd, syscall.EPOLL_CTL_DEL, fd, &syscall.EpollEvent{Fd: int32(fd)})
}

// 事件循环
func (e *eventPollState) Poll(tv time.Duration, cb func(fd int, state State, err error)) (numEvents int, err error) {
	msec := -1
	if tv > 0 {
		msec = int(tv) / int(time.Millisecond)
	}

	numEvents, err = syscall.EpollWait(e.epfd, e.events, msec)
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
