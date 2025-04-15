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
package pulse

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const ptrSize = 4 << (^uintptr(0) >> 63)

type safeConns[T any] struct {
	mu       sync.Mutex
	conns    []*T
	connsPtr **T
	len      uintptr
}

func (s *safeConns[T]) init() {
	s.conns = make([]*T, 1000000) // 100w个指针
}

func (s *safeConns[T]) Add(fd int, c *T) {
	if fd == -1 {
		return
	}

	s.mu.Lock()
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
	s.mu.Unlock()
}

func add(base unsafe.Pointer, index uintptr) unsafe.Pointer {
	return unsafe.Pointer(uintptr(base) + index*ptrSize)
}

func (s *safeConns[T]) addInner(fd int, c *T) {

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

func (s *safeConns[T]) Del(fd int) {

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

func (s *safeConns[T]) Get(fd int) *T {
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
