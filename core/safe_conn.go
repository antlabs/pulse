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
package core

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const ptrSize = 4 << (^uintptr(0) >> 63)

type SafeConns[T any] struct {
	mu       sync.Mutex
	conns    []*T
	connsPtr **T
	len      uintptr
}

func (s *SafeConns[T]) Init(max int) {
	s.conns = make([]*T, max)
	s.connsPtr = &s.conns[0]
}

func (s *SafeConns[T]) Add(fd int, c *T) {
	if fd == -1 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if fd >= len(s.conns) {
		if fd >= cap(s.conns) {
			newConns := make([]*T, max(int(float64(len(s.conns))*1.25), fd+1))
			copy(newConns, s.conns)
			s.conns = newConns
		} else {
			s.conns = s.conns[:cap(s.conns)+1]
		}
	}
	if s.connsPtr != &s.conns[0] {
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&s.connsPtr)), unsafe.Pointer(&s.conns[0]))
	}
	atomic.StoreUintptr(&s.len, uintptr(len(s.conns)))

	s.addInner(fd, c)
}

func add(base unsafe.Pointer, index uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(base) + index*ptrSize)
}

func (s *SafeConns[T]) addInner(fd int, c *T) {

	if fd == -1 {
		return
	}

	l := atomic.LoadUintptr(&s.len)
	if int(fd) > int(l) {
		return
	}

	atomic.StorePointer((*unsafe.Pointer)(
		add(atomic.LoadPointer(
			(*unsafe.Pointer)((unsafe.Pointer)(&s.connsPtr))),
			uintptr(fd))),
		unsafe.Pointer(c))
}

func (s *SafeConns[T]) Del(fd int) {

	if fd == -1 {
		return
	}

	l := atomic.LoadUintptr(&s.len)
	if int(fd) > int(l) {
		return
	}

	atomic.StorePointer((*unsafe.Pointer)(
		add(atomic.LoadPointer(
			(*unsafe.Pointer)((unsafe.Pointer)(&s.connsPtr))),
			uintptr(fd))),
		nil)
}

func (s *SafeConns[T]) GetUnsafe(fd int) *T {
	return s.conns[fd]
}

func (s *SafeConns[T]) Get(fd int) *T {
	if fd == -1 {
		return nil
	}

	l := atomic.LoadUintptr(&s.len)
	if int(fd) > int(l) {
		return nil
	}

	return (*T)(atomic.LoadPointer((*unsafe.Pointer)(
		add(
			atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&s.connsPtr))),
			uintptr(fd)))))
}

func (s *SafeConns[T]) UnsafeConns() []*T {
	return s.conns
}
