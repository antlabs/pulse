//go:build darwin

package cpu

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
)

// 使用 go-purego 库加载动态库

// func main() {
// 	// 测试本机 CPU 使用率
// 	fmt.Println("=== 本机 CPU 使用率测试 ===")
// 	cpuPercent, err := PercentWithContext(context.Background(), 1*time.Second, false)
// 	if err != nil {
// 		fmt.Printf("错误: %v\n", err)
// 	} else {
// 		fmt.Printf("本机 CPU 使用率: %.2f%%\n", cpuPercent[0])
// 	}

// 	// 测试本进程 CPU 使用率
// 	fmt.Println("\n=== 本进程 CPU 使用率测试 ===")
// 	p := &Process{Pid: 0} // 0 表示当前进程
// 	procPercent, err := p.PercentWithContext(context.Background(), 1*time.Second)
// 	if err != nil {
// 		fmt.Printf("错误: %v\n", err)
// 	} else {
// 		fmt.Printf("本进程 CPU 使用率: %.2f%%\n", procPercent)
// 	}
// }

// CPU时间统计结构体
type TimesStat struct {
	CPU       string  // CPU 标识符
	User      float64 // 用户模式时间
	System    float64 // 系统模式时间
	Idle      float64 // 空闲时间
	Nice      float64 // 优先级调整时间
	Iowait    float64 // IO 等待时间
	Irq       float64 // 硬中断时间
	Softirq   float64 // 软中断时间
	Steal     float64 // 虚拟化环境中的空闲时间
	Guest     float64 // 运行虚拟 CPU 的时间
	GuestNice float64 // 运行低优先级虚拟 CPU 的时间
}

// 计算总时间
func (t *TimesStat) Total() float64 {
	return t.User + t.System + t.Idle + t.Nice + t.Iowait + t.Irq + t.Softirq + t.Steal + t.Guest + t.GuestNice
}

// 用于跟踪上次 CPU 使用率的计算
type lastPercent struct {
	sync.Mutex
	lastCPUTimes    []TimesStat
	lastPerCPUTimes []TimesStat
}

var lastCPUPercent lastPercent

// 进程结构体
type Process struct {
	Pid          int32
	lastCPUTimes *TimesStat
	lastCPUTime  time.Time
}

// 系统时钟频率，默认128Hz
var clocksPerSec = float64(128)

// 动态库结构体
type Library struct {
	handle uintptr
	path   string
}

// Mach 常量
const (
	KERN_SUCCESS = 0

	// CPU 状态
	CPU_STATE_USER   = 0
	CPU_STATE_SYSTEM = 1
	CPU_STATE_IDLE   = 2
	CPU_STATE_NICE   = 3

	// Mach 主机信息
	HOST_CPU_LOAD_INFO       = 3
	HOST_CPU_LOAD_INFO_COUNT = 4

	// 进程信息
	PROC_PIDTASKINFO = 4
)

// CPU 负载信息结构体
type hostCpuLoadInfoData struct {
	cpuTicks [4]uint32
}

// 进程任务信息结构体
type procTaskInfo struct {
	Virtual_size      uint64
	Resident_size     uint64
	Total_user        uint64
	Total_system      uint64
	Threads_user      uint64
	Threads_system    uint64
	Policy            int32
	Faults            int32
	Pageins           int32
	Cow_faults        int32
	Messages_sent     int32
	Messages_received int32
	Syscalls_mach     int32
	Syscalls_unix     int32
	Csw               int32
	Threadnum         int32
	Numrunning        int32
	Priority          int32
}

// 初始化函数
func init() {
	// 获取系统时钟频率
	lib, err := loadLibrary("/usr/lib/libSystem.B.dylib")
	//lib, err := loadLibrary("/usr/lib/libSystem.dylib")
	if err != nil {
		return
	}
	defer lib.Close()

	// 获取 sysctl 函数
	sysctlFunc, err := lib.GetSymbol("sysctl")
	if err != nil {
		return
	}

	// 调用 sysctl 获取时钟频率
	var clockrate struct {
		Hz int32
	}
	mib := []int32{6, 7} // CTL_KERN, KERN_CLOCKRATE
	size := uint64(unsafe.Sizeof(clockrate))

	// sysctlFn := purego.NewCallback(sysctlFunc)
	status, _, _ := purego.SyscallN(sysctlFunc,
		uintptr(unsafe.Pointer(&mib[0])),
		uintptr(2),
		uintptr(unsafe.Pointer(&clockrate)),
		uintptr(unsafe.Pointer(&size)),
		0, 0)

	if status == 0 && clockrate.Hz > 0 {
		clocksPerSec = float64(clockrate.Hz)
	}
}

// 加载动态库
func loadLibrary(path string) (*Library, error) {
	handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		return nil, fmt.Errorf("failed to load library %s: %v", path, err)
	}

	return &Library{
		handle: handle,
		path:   path,
	}, nil
}

// 获取符号
func (l *Library) GetSymbol(name string) (uintptr, error) {
	sym, err := purego.Dlsym(l.handle, name)
	if err != nil {
		return 0, fmt.Errorf("failed to find symbol %s: %v", name, err)
	}
	return sym, nil
}

// 关闭动态库
func (l *Library) Close() {
	purego.Dlclose(l.handle)
}

// 获取本机CPU使用率
func PercentWithContext(ctx context.Context, interval time.Duration, percpu bool) ([]float64, error) {
	if interval <= 0 {
		return percentUsedFromLastCallWithContext(ctx, percpu)
	}

	// 获取CPU使用率的起始时间点
	cpuTimes1, err := TimesWithContext(ctx, percpu)
	if err != nil {
		return nil, err
	}

	// 等待指定的时间间隔
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(interval):
	}

	// 获取CPU使用率的结束时间点
	cpuTimes2, err := TimesWithContext(ctx, percpu)
	if err != nil {
		return nil, err
	}

	return calculateAllBusy(cpuTimes1, cpuTimes2)
}

// 使用上次调用的数据计算CPU使用率
func percentUsedFromLastCallWithContext(ctx context.Context, percpu bool) ([]float64, error) {
	cpuTimes, err := TimesWithContext(ctx, percpu)
	if err != nil {
		return nil, err
	}

	lastCPUPercent.Lock()
	defer lastCPUPercent.Unlock()

	var lastTimes []TimesStat
	if percpu {
		lastTimes = lastCPUPercent.lastPerCPUTimes
		lastCPUPercent.lastPerCPUTimes = cpuTimes
	} else {
		lastTimes = lastCPUPercent.lastCPUTimes
		lastCPUPercent.lastCPUTimes = cpuTimes
	}

	if lastTimes == nil {
		return nil, fmt.Errorf("error getting times for cpu percent. lastTimes was nil")
	}

	return calculateAllBusy(lastTimes, cpuTimes)
}

// 计算所有CPU的忙碌百分比
func calculateAllBusy(t1, t2 []TimesStat) ([]float64, error) {
	if len(t1) != len(t2) {
		return nil, fmt.Errorf("received two CPU counts: %d != %d", len(t1), len(t2))
	}

	ret := make([]float64, len(t1))
	for i, t := range t2 {
		ret[i] = calculateBusy(t1[i], t)
	}
	return ret, nil
}

// 计算单个CPU的忙碌百分比
func calculateBusy(t1, t2 TimesStat) float64 {
	t1All, t1Busy := getAllBusy(t1)
	t2All, t2Busy := getAllBusy(t2)

	if t2Busy <= t1Busy {
		return 0
	}
	if t2All <= t1All {
		return 100
	}
	return math.Min(100, math.Max(0, (t2Busy-t1Busy)/(t2All-t1All)*100))
}

// 获取CPU的总时间和忙碌时间
func getAllBusy(t TimesStat) (float64, float64) {
	tot := t.Total()
	busy := tot - t.Idle - t.Iowait
	return tot, busy
}

// 获取CPU时间统计信息
func TimesWithContext(ctx context.Context, percpu bool) ([]TimesStat, error) {
	lib, err := loadLibrary("/usr/lib/libSystem.dylib")
	if err != nil {
		return nil, err
	}
	defer lib.Close()

	if percpu {
		return perCPUTimes(lib)
	}

	return allCPUTimes(lib)
}

// 获取所有CPU核心的时间统计
func allCPUTimes(lib *Library) ([]TimesStat, error) {
	// 获取 mach_host_self 函数
	machHostSelfSym, err := lib.GetSymbol("mach_host_self")
	if err != nil {
		return nil, err
	}

	// 获取 host_statistics 函数
	hostStatisticsSym, err := lib.GetSymbol("host_statistics")
	if err != nil {
		return nil, err
	}

	// 调用 mach_host_self
	// machHostSelfFn := purego.NewCallback(machHostSelfSym)
	hostSelf, _, _ := purego.SyscallN(machHostSelfSym)

	// 准备调用 host_statistics
	var cpuload hostCpuLoadInfoData
	count := uint32(HOST_CPU_LOAD_INFO_COUNT)

	// hostStatisticsFn := purego.NewCallback(hostStatisticsSym)
	status, _, _ := purego.SyscallN(hostStatisticsSym,
		hostSelf,
		uintptr(HOST_CPU_LOAD_INFO),
		uintptr(unsafe.Pointer(&cpuload)),
		uintptr(unsafe.Pointer(&count)))

	if status != KERN_SUCCESS {
		return nil, fmt.Errorf("host_statistics error=%d", status)
	}

	c := TimesStat{
		CPU:    "cpu-total",
		User:   float64(cpuload.cpuTicks[CPU_STATE_USER]) / clocksPerSec,
		System: float64(cpuload.cpuTicks[CPU_STATE_SYSTEM]) / clocksPerSec,
		Nice:   float64(cpuload.cpuTicks[CPU_STATE_NICE]) / clocksPerSec,
		Idle:   float64(cpuload.cpuTicks[CPU_STATE_IDLE]) / clocksPerSec,
	}

	return []TimesStat{c}, nil
}

// 获取每个CPU核心的时间统计
func perCPUTimes(lib *Library) ([]TimesStat, error) {
	// 在macOS上，我们可以使用 host_processor_info 获取每个CPU核心的信息
	// 但这需要更复杂的实现，这里简化为复制总体CPU使用率到每个核心

	stats, err := allCPUTimes(lib)
	if err != nil {
		return nil, err
	}

	numCPU := runtime.NumCPU()
	ret := make([]TimesStat, numCPU)

	for i := 0; i < numCPU; i++ {
		ret[i] = TimesStat{
			CPU:    fmt.Sprintf("cpu%d", i),
			User:   stats[0].User / float64(numCPU),
			System: stats[0].System / float64(numCPU),
			Nice:   stats[0].Nice / float64(numCPU),
			Idle:   stats[0].Idle / float64(numCPU),
		}
	}

	return ret, nil
}

// 获取进程CPU使用率
func (p *Process) PercentWithContext(ctx context.Context, interval time.Duration) (float64, error) {
	cpuTimes, err := p.TimesWithContext(ctx)
	if err != nil {
		return 0, err
	}
	now := time.Now()

	if interval > 0 {
		p.lastCPUTimes = cpuTimes
		p.lastCPUTime = now

		// 等待指定的时间间隔
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(interval):
		}

		cpuTimes, err = p.TimesWithContext(ctx)
		now = time.Now()
		if err != nil {
			return 0, err
		}
	} else if p.lastCPUTimes == nil {
		// 首次调用
		p.lastCPUTimes = cpuTimes
		p.lastCPUTime = now
		return 0, nil
	}

	numcpu := runtime.NumCPU()
	delta := (now.Sub(p.lastCPUTime).Seconds()) * float64(numcpu)
	ret := calculatePercent(p.lastCPUTimes, cpuTimes, delta, numcpu)
	p.lastCPUTimes = cpuTimes
	p.lastCPUTime = now
	return ret, nil
}

// 计算进程CPU使用率百分比
func calculatePercent(t1, t2 *TimesStat, delta float64, numcpu int) float64 {
	if delta == 0 {
		return 0
	}
	deltaProc := (t2.User - t1.User) + (t2.System - t1.System)
	if deltaProc <= 0 {
		return 0
	}
	overallPercent := ((deltaProc / delta) * 100) * float64(numcpu)
	return overallPercent
}

// 获取进程CPU时间统计信息
func (p *Process) TimesWithContext(ctx context.Context) (*TimesStat, error) {
	lib, err := loadLibrary("/usr/lib/libSystem.dylib")
	if err != nil {
		return nil, err
	}
	defer lib.Close()

	// 获取 proc_pidinfo 函数
	procPidInfoSym, err := lib.GetSymbol("proc_pidinfo")
	if err != nil {
		return nil, err
	}

	// 获取时间基准信息
	machTimeBaseInfoSym, err := lib.GetSymbol("mach_timebase_info")
	if err != nil {
		return nil, err
	}

	// 获取当前进程ID
	pid := p.Pid
	if pid == 0 {
		pid = int32(os.Getpid())
	}

	// 获取进程任务信息
	var ti procTaskInfo
	// procPidInfoFn := purego.NewCallback(procPidInfoSym)
	ret, _, _ := purego.SyscallN(procPidInfoSym,
		uintptr(pid),
		uintptr(PROC_PIDTASKINFO),
		uintptr(0),
		uintptr(unsafe.Pointer(&ti)),
		uintptr(unsafe.Sizeof(ti)))

	if ret <= 0 {
		return nil, errors.New("proc_pidinfo failed")
	}

	// 获取时间基准信息
	type timebaseInfo struct {
		Numer uint32
		Denom uint32
	}
	var tbi timebaseInfo

	// machTimeBaseInfoFn := purego.NewCallback(machTimeBaseInfoSym)
	purego.SyscallN(machTimeBaseInfoSym, uintptr(unsafe.Pointer(&tbi)))

	// 计算时间比例
	timeScale := float64(tbi.Numer) / float64(tbi.Denom)

	// 创建CPU时间统计
	return &TimesStat{
		CPU:    "cpu",
		User:   float64(ti.Total_user) * timeScale / 1e9,
		System: float64(ti.Total_system) * timeScale / 1e9,
	}, nil
}

func GetCPUInfo() (CPUInfo, error) {
	cpuTimes, err := TimesWithContext(context.Background(), false)
	if err != nil {
		return CPUInfo{}, err
	}

	allUser := 0.0
	allSystem := 0.0
	allIdle := 0.0
	allTotal := 0.0
	for _, cpu := range cpuTimes {
		allUser += cpu.User
		allSystem += cpu.System
		allIdle += cpu.Idle
		allTotal += cpu.Total()
	}
	return CPUInfo{
		User:   allUser,
		System: allSystem,
		Idle:   allIdle,
		Total:  allTotal,
	}, nil
}

func GetProcessCPUInfo(pid int) (ProcessCPUInfo, error) {
	p := &Process{Pid: 0} // 0 表示当前进程
	pPercent, err := p.TimesWithContext(context.Background())
	if err != nil {
		return ProcessCPUInfo{}, err
	}
	numCPU := float64(runtime.NumCPU())
	adjust := 10.0
	return ProcessCPUInfo{
		User:   (pPercent.User / numCPU) / adjust,
		System: (pPercent.System / numCPU) / adjust,
		Total:  (pPercent.Total() / numCPU) / adjust,
	}, nil
}
