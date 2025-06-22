//go:build windows
// +build windows

package core

import (
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	createIoCompletionPort     = kernel32.NewProc("CreateIoCompletionPort")
	postQueuedCompletionStatus = kernel32.NewProc("PostQueuedCompletionStatus")
	getQueuedCompletionStatus  = kernel32.NewProc("GetQueuedCompletionStatus")
	closeHandle                = kernel32.NewProc("CloseHandle")
)

type iocp struct {
	handle windows.Handle
	events map[int]State
}

func Create(triggerType TriggerType) (PollingApi, error) {
	handle, _, err := createIoCompletionPort.Call(
		uintptr(windows.InvalidHandle),
		0,
		0,
		0,
	)
	if handle == 0 {
		return nil, err
	}

	return &iocp{
		handle: windows.Handle(handle),
		events: make(map[int]State),
	}, nil
}

func (i *iocp) AddRead(fd int) error {
	i.events[fd] |= READ
	return nil
}

func (i *iocp) AddWrite(fd int) error {
	i.events[fd] |= WRITE
	return nil
}

func (i *iocp) ResetRead(fd int) error {
	i.events[fd] &^= READ
	return nil
}

func (i *iocp) DelRead(fd int) error {
	i.events[fd] &^= READ
	return nil
}

func (i *iocp) Del(fd int) error {
	delete(i.events, fd)
	return nil
}

func (i *iocp) Poll(tv time.Duration, cb func(int, State, error)) (retVal int, err error) {
	var bytes uint32
	var key uint32
	var overlapped *windows.Overlapped
	var timeout uint32

	if tv > 0 {
		timeout = uint32(tv.Milliseconds())
	} else {
		timeout = windows.INFINITE
	}

	ret, _, err := getQueuedCompletionStatus.Call(
		uintptr(i.handle),
		uintptr(unsafe.Pointer(&bytes)),
		uintptr(unsafe.Pointer(&key)),
		uintptr(unsafe.Pointer(&overlapped)),
		uintptr(timeout),
	)

	if ret == 0 {
		if err == windows.ERROR_ABANDONED_WAIT_0 {
			return 0, nil
		}
		return 0, err
	}

	fd := int(key)
	state := i.events[fd]
	if state == 0 {
		return 0, nil
	}

	cb(fd, state, nil)
	return 1, nil
}

func (i *iocp) Free() {
	if i.handle != windows.InvalidHandle {
		closeHandle.Call(uintptr(i.handle))
		i.handle = windows.InvalidHandle
	}
}

func (i *iocp) Name() string {
	return "iocp"
}
