package core

import "time"

// 枚举变量状态有可写，写读
type State uint32

const (
	// 可写
	WRITE State = 1 << iota
	// 可读
	READ
)

func (s State) IsWrite() bool {
	return s&WRITE != 0
}

func (s State) IsRead() bool {
	return s&READ != 0
}

type PollingApi interface {
	AddRead(fd int) error
	AddWrite(fd int) error
	Del(fd int) error
	Poll(tv time.Duration, cb func(int, State, error)) (retVal int, err error)
	Free()
	Name() string
}
