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
	defMaxSocketReadTimes      = 1
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
	callback                   Callback         // 回调函数
	task                       taskConfig       // 协程池配置
	level                      *slog.Level      // 日志级别
	taskType                   TaskType         // 任务类型
	triggerType                core.TriggerType // 触发类型, 水平触发还是边缘触发
	eventLoopReadBufferSize    int              // event loop中读buffer的大小
	maxSocketReadTimes         int              // socket单次最大读取次数
	flowBackPressure           bool             // 流量背压机制，当连接的写缓冲区满了，会暂停读取，直到写缓冲区有空闲空间
	flowBackPressureRemoveRead bool             // 流量背压机制，当连接的写缓冲区满了，会移除读事件，直到写缓冲区有空闲空间
}

// 单次可读事情，最大读取次数(水平触发模式有效)
func WithMaxSocketReadTimes(maxSocketReadTimes int) func(*Options) {
	return func(o *Options) {
		o.maxSocketReadTimes = maxSocketReadTimes
	}
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
		o.level = &level
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

// 设置流量背压机制，当连接的写缓冲区满了，会暂停读取，直到写缓冲区有空闲空间
func WithFlowBackPressure(enable bool) func(*Options) {
	return func(o *Options) {
		o.flowBackPressure = enable
	}
}

// 设置流量背压机制，当连接的写缓冲区满了，会移除读事件，直到写缓冲区有空闲空间
func WithFlowBackPressureRemoveRead(enable bool) func(*Options) {
	return func(o *Options) {
		o.flowBackPressureRemoveRead = enable
	}
}
