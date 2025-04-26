package pulse

import "runtime"

var (
	defMaxEventNum   = 256
	defTaskMin       = 50
	defTaskMax       = 30000
	defTaskInitCount = 8
	defNumLoops      = runtime.NumCPU()
)

type taskConfig struct {
	initCount int // 初始化的协程数
	min       int // 最小协程数
	max       int // 最大协程数
}
type Options[T any] struct {
	callback Callback[T]
	decoder  Decoder[T]
	task     taskConfig
}

func WithCallback[T any](callback Callback[T]) func(*Options[T]) {
	return func(o *Options[T]) {
		o.callback = callback
	}
}

func WithDecoder[T any](decoder Decoder[T]) func(*Options[T]) {

	return func(o *Options[T]) {
		o.decoder = decoder
	}
}
