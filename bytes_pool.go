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
		pools = append(pools, sync.Pool{
			New: func() interface{} {
				buf := make([]byte, j*page)
				return &buf
			},
		})
	}
}

const (
	page     = 1024
	maxIndex = 64
)

var pools = make([]sync.Pool, 0, maxIndex)

func selectIndex(n int) int {
	index := n / page
	return index
}

func getBytes(n int) (rv *[]byte) {

	index := selectIndex(n)
	if index >= len(pools) {
		// TODO 优化下
		rv := make([]byte, n)
		return &rv
	}

	rv = pools[index].Get().(*[]byte)
	*rv = (*rv)[:cap(*rv)]
	return rv
}

func putBytes(bytes *[]byte) {
	if cap(*bytes) == 0 {
		return
	}
	if cap(*bytes) < page {
		return
	}

	newLen := cap(*bytes) - 1
	index := selectIndex(newLen)
	if (cap(*bytes))%page != 0 {
		index--
		if index < 0 {
			return
		}
	}
	if index >= len(pools) {
		return
	}
	pools[index].Put(bytes)
}
