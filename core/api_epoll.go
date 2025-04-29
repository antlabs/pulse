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
	"time"

	"golang.org/x/sys/unix"
)

const (
	// 垂直触发
	// 来自man 手册
	// When  used as an edge-triggered interface, for performance reasons,
	// it is possible to add the file descriptor inside the epoll interface (EPOLL_CTL_ADD) once by specifying (EPOLLIN|EPOLLOUT).
	// This allows you to avoid con‐
	// tinuously switching between EPOLLIN and EPOLLOUT calling epoll_ctl(2) with EPOLL_CTL_MOD.
	etRead      = uint32(unix.EPOLLERR | unix.EPOLLHUP | unix.EPOLLRDHUP | unix.EPOLLPRI | unix.EPOLLIN | unix.EPOLLOUT | unix.EPOLLET)
	etWrite     = uint32(0)
	etDelWrite  = uint32(0)
	etResetRead = uint32(0)

	// TODO
	ltRead      = uint32(0)
	ltWrite     = uint32(0)
	ltDelWrite  = uint32(0)
	ltResetRead = uint32(0)

	// 写事件
	processWrite = uint32(unix.EPOLLOUT)
	// 读事件
	processRead = uint32(unix.EPOLLIN | unix.EPOLLRDHUP | unix.EPOLLHUP | unix.EPOLLERR)
)

var _ PollingApi = (*eventPollState)(nil)

type eventPollState struct {
	epfd   int
	events []unix.EpollEvent

	et      bool
	rev     uint32
	wev     uint32
	dwEv    uint32 // delete write event
	resetEv uint32
}

func getReadWriteDeleteReset(et bool) (uint32, uint32, uint32, uint32) {
	if et {
		return etRead, etWrite, etDelWrite, etResetRead
	}

	return ltRead, ltWrite, ltDelWrite, ltResetRead
}

// 创建epoll handler
func Create() (la PollingApi, err error) {
	var e eventPollState
	e.epfd, err = unix.EpollCreate1(0)
	if err != nil {
		return nil, err
	}

	e.events = make([]unix.EpollEvent, 128)
	e.rev, e.wev, e.dwEv, e.resetEv = getReadWriteDeleteReset(true)
	return &e, nil
}

// 释放
func (e *eventPollState) Free() {
	unix.Close(e.epfd)
}

// 新加读事件
func (e *eventPollState) AddRead(fd int) error {
	if e.rev > 0 && fd >= 0 {
		return unix.EpollCtl(e.epfd, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{
			Fd:     int32(fd),
			Events: e.rev,
		})
	}
	return nil
}

// 新加写事件
func (e *eventPollState) AddWrite(fd int) error {
	if e.wev > 0 && fd >= 0 {
		return unix.EpollCtl(e.epfd, unix.EPOLL_CTL_MOD, fd, &unix.EpollEvent{
			Fd:     int32(fd),
			Events: e.wev,
		})
	}

	return nil
}

func (e *eventPollState) ResetRead(fd int) error {
	if e.resetEv > 0 && fd >= 0 {
		return unix.EpollCtl(e.epfd, unix.EPOLL_CTL_MOD, fd, &unix.EpollEvent{
			Fd:     int32(fd),
			Events: e.resetEv,
		})
	}
	return nil
}

// 删除写事件
func (e *eventPollState) DelWrite(fd int) error {
	if e.dwEv > 0 {
		return unix.EpollCtl(e.epfd, unix.EPOLL_CTL_MOD, fd, &unix.EpollEvent{
			Fd:     int32(fd),
			Events: e.dwEv,
		})
	}
	return nil
}

// 删除事件
func (e *eventPollState) Del(fd int) error {
	return unix.EpollCtl(e.epfd, unix.EPOLL_CTL_DEL, fd, &unix.EpollEvent{Fd: int32(fd)})
}

// 事件循环
func (e *eventPollState) Poll(tv time.Duration, cb func(fd int, state State, err error)) (retVal int, err error) {
	msec := -1
	if tv > 0 {
		msec = int(tv) / int(time.Millisecond)
	}

	retVal, err = unix.EpollWait(e.epfd, e.events, msec)
	if err != nil {
		if errors.Is(err, unix.EINTR) {
			return 0, nil
		}
		return 0, err
	}
	numEvents := 0
	if retVal > 0 {
		numEvents = retVal
		for i := 0; i < numEvents; i++ {
			ev := &e.events[i]
			fd := ev.Fd

			// unix.EPOLLRDHUP是关闭事件，遇到直接关闭
			if ev.Events&(unix.EPOLLERR|unix.EPOLLHUP|unix.EPOLLRDHUP) > 0 {
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

	}

	return numEvents, nil
}

func (e *eventPollState) Name() string {
	return "epoll"
}
