// Copyright 2023-2025 antlabs. All rights reserved.
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

//go:build darwin || freebsd
// +build darwin freebsd

package core

import (
	"errors"
	"io"
	"time"

	"golang.org/x/sys/unix"
)

var _ PollingApi = (*eventPollState)(nil)

type eventPollState struct {
	kqfd   int
	events []unix.Kevent_t
}

func Create() (as PollingApi, err error) {
	var state eventPollState
	state.kqfd, err = unix.Kqueue()
	if err != nil {
		return nil, err
	}
	state.events = make([]unix.Kevent_t, 1024)

	return &state, nil
}

// 新加读事件
func (as *eventPollState) AddRead(fd int) error {
	if fd == -1 {
		return nil
	}

	_, err := unix.Kevent(as.kqfd, []unix.Kevent_t{
		{Ident: uint64(fd), Flags: unix.EV_ADD, Filter: unix.EVFILT_READ},
		{Ident: uint64(fd), Flags: unix.EV_ADD, Filter: unix.EVFILT_WRITE},
	}, nil, nil)
	return err
}

// 新加写事件
func (as *eventPollState) AddWrite(fd int) error {
	return nil
}

func (as *eventPollState) Del(fd int) error {
	// _, err := unix.Kevent(as.kqfd, []unix.Kevent_t{
	// 	{Ident: uint64(fd), Flags: unix.EV_DELETE, Filter: unix.EVFILT_READ},
	// 	{Ident: uint64(fd), Flags: unix.EV_DELETE, Filter: unix.EVFILT_WRITE},
	// }, nil, nil)
	// return err
	return nil
}

func (as *eventPollState) Poll(tv time.Duration, cb func(int, State, error)) (retVal int, err error) {
	var timeout *unix.Timespec
	if tv >= 0 {
		var tempTimeout unix.Timespec
		tempTimeout.Sec = int64(tv / time.Second)
		tempTimeout.Nsec = int64(tv % time.Second)
		timeout = &tempTimeout
	}

	retVal, err = unix.Kevent(as.kqfd, nil, as.events, timeout)
	if err != nil {
		if errors.Is(err, unix.EINTR) {
			return 0, nil
		}
		return 0, err
	}

	if retVal > 0 {
		for j := 0; j < retVal; j++ {
			ev := &as.events[j]
			fd := int(ev.Ident)

			if ev.Flags&unix.EV_EOF != 0 {
				cb(fd, WRITE, io.EOF)
				continue
			}

			isRead := ev.Filter == unix.EVFILT_READ
			isWrite := ev.Filter == unix.EVFILT_WRITE
			if isRead {
				cb(fd, READ, nil)
				if err != nil {
					cb(fd, WRITE, io.EOF)
					continue
				}
			}

			if isWrite {
				// 刷新下直接写入失败的数据
				cb(fd, WRITE, nil)
			}

		}
	}
	return retVal, nil
}

func (as *eventPollState) Free() {
	if as != nil {
		unix.Close(as.kqfd)
		as.kqfd = -1
	}
}

func (as *eventPollState) Name() string {
	return "kqueue"
}
