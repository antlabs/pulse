package pulse

import (
	"log/slog"
	"runtime"

	"github.com/antlabs/pulse/core"
)

var (
	defMaxEventNum   = 256
	defTaskMin       = 50
	defTaskMax       = 30000
	defTaskInitCount = 8
	defNumLoops      = runtime.NumCPU()
)

type TaskType int

const (
	// 在业务协程中执行
	TaskTypeInBusinessGoroutine TaskType = iota
	// 在event loop中执行
	TaskTypeInEventLoop
	// 一个连接独占一个协程
	TaskTypeInConnectionGoroutine
)

// 水平触发
const TriggerTypeLevel = core.TriggerTypeLevel

// 边缘触发
const TriggerTypeEdge = core.TriggerTypeEdge

type taskConfig struct {
	initCount int // 初始化的协程数
	min       int // 最小协程数
	max       int // 最大协程数
}

// 边缘触发
type Options[T any] struct {
	callback    Callback[T]
	decoder     Decoder[T]
	task        taskConfig
	level       slog.Level
	taskType    TaskType
	triggerType core.TriggerType
}

// 设置回调函数
func WithCallback[T any](callback Callback[T]) func(*Options[T]) {
	return func(o *Options[T]) {
		o.callback = callback
	}
}

// 设置解码器
func WithDecoder[T any](decoder Decoder[T]) func(*Options[T]) {

	return func(o *Options[T]) {
		o.decoder = decoder
	}
}

// 设置日志级别
func WithLogLevel[T any](level slog.Level) func(*Options[T]) {
	return func(o *Options[T]) {
		o.level = level
	}
}

// 选择task的类型
func WithTaskType[T any](taskType TaskType) func(*Options[T]) {
	return func(o *Options[T]) {
		o.taskType = taskType
	}
}

// 设置水平触发还是边缘触发
func WithTriggerType[T any](triggerType core.TriggerType) func(*Options[T]) {
	return func(o *Options[T]) {
		o.triggerType = triggerType
	}
}
