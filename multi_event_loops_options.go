package pulse

import (
	"log/slog"

	"github.com/antlabs/pulse/core"
)

var (
	defTaskMin                 = 50
	defTaskMax                 = 30000
	defTaskInitCount           = 8
	defEventLoopReadBufferSize = 1024 * 4
)

type TaskType int

const (
	// 在业务协程池中执行
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
type Options struct {
	callback                Callback
	task                    taskConfig
	level                   slog.Level
	taskType                TaskType
	triggerType             core.TriggerType
	eventLoopReadBufferSize int
}

// 设置回调函数
func WithCallback(callback Callback) func(*Options) {
	return func(o *Options) {
		o.callback = callback
	}
}

// 设置日志级别
func WithLogLevel(level slog.Level) func(*Options) {
	return func(o *Options) {
		o.level = level
	}
}

// 选择task的类型
func WithTaskType(taskType TaskType) func(*Options) {
	return func(o *Options) {
		o.taskType = taskType
	}
}

// 设置水平触发还是边缘触发
func WithTriggerType(triggerType core.TriggerType) func(*Options) {
	return func(o *Options) {
		o.triggerType = triggerType
	}
}

// 设置event loop里面读buffer的大小
func WithEventLoopReadBufferSize(size int) func(*Options) {
	return func(o *Options) {
		o.eventLoopReadBufferSize = size
	}
}
