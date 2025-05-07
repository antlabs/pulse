package pulse

// Copyright 2021-2023 antlabs. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"log/slog"
	"strconv"
	"sync"
)

// 生成的大小分别是
// 1 * 1024   = 1024
// 2 * 1024   = 2048
// 3 * 1024   = 3072
// 4 * 1024   = 4096
// 5 * 1024   = 5120
// 6 * 1024   = 6144
// 7 * 1024   = 7182
func init() {
	for i := 1; i <= maxIndex; i++ {
		j := i
		smallPools = append(smallPools, sync.Pool{
			New: func() interface{} {
				buf := make([]byte, j*page)
				return &buf
			},
		})
	}
}

const (
	page        = 1024
	maxIndex    = 256
	minPoolSize = page * maxIndex
)

// 小缓存池 1kb 2kb 3kb 4kb 5kb 6kb 7kb... 256kb
var smallPools = make([]sync.Pool, 0, maxIndex)

func selectIndex(n int) int {
	index := n / page
	return index
}

func getBytes(n int) (rv *[]byte) {

	index := selectIndex(n - 1)
	if index >= len(smallPools) {
		rv = getBigBytes(n)
		i := 0
		for i < 3 {
			if cap(*rv) >= n {
				return rv
			}
			rv = getBigBytes(n)
			i++
		}
		if i == 3 {
			panic("getBytes getBigBytes failed, need size:" + strconv.Itoa(n) + " got size:" + strconv.Itoa(cap(*rv)))
		}
		return rv
	}

	rv = smallPools[index].Get().(*[]byte)
	*rv = (*rv)[:cap(*rv)]
	return rv
}

func putBytes(bytes *[]byte) {
	if bytes == nil || cap(*bytes) == 0 {
		return
	}
	if cap(*bytes) < page {
		return
	}

	newLen := cap(*bytes) - 1
	index := selectIndex(newLen)
	if index >= len(smallPools) {
		putBigBytes(bytes)
		return
	}

	// 如果cap(*bytes)%page != 0 ，说明append的时候扩容了
	if cap(*bytes)%page != 0 {
		index-- // 向前挪一格, 可以保证空间是够的
	}
	smallPools[index].Put(bytes)
}

// 大缓存池 256kb 512kb 1mb
// 生效的概率是比较低的
var bigPools = make([]sync.Pool, 0, 4)
var bigPoolsSize = []int{
	512 * 1024,
	1024 * 1024,
	2 * 1024 * 1024,
	3 * 1024 * 1024,
	4 * 1024 * 1024,
	5 * 1024 * 1024,
	6 * 1024 * 1024,
	7 * 1024 * 1024,
	8 * 1024 * 1024,
}

func init() {
	for i := range bigPoolsSize {
		bigPools = append(bigPools, sync.Pool{
			New: func() interface{} {
				buf := make([]byte, bigPoolsSize[i])
				return &buf
			},
		})
	}
}

func getBigBytes(n int) (rv *[]byte) {
	if n <= bigPoolsSize[len(bigPoolsSize)-1] {
		for i := range bigPoolsSize {
			if n <= bigPoolsSize[i] {
				rv = bigPools[i].Get().(*[]byte)
				*rv = (*rv)[:cap(*rv)]
				return rv
			}
		}
		return
	}

	if n < minPoolSize {
		panic("n is too small")
	}

	rv2 := make([]byte, n)
	slog.Info("getBigBytes make([]byte, n)", "n", n)

	return &rv2
}

func putBigBytes(bytes *[]byte) {
	if bytes == nil || cap(*bytes) == 0 {
		return
	}

	if cap(*bytes) < minPoolSize {
		return
	}

	for i := range bigPoolsSize {
		if cap(*bytes) <= bigPoolsSize[i] {
			bigPools[i].Put(bytes)
			return
		}
	}

	// panic("putBigBytes failed, need size:" + strconv.Itoa(cap(*bytes)))
}
