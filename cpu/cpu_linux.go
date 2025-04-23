package cpu

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// 获取 Linux 系统的 CPU 信息
func GetCPUInfo() (CPUInfo, error) {
	data, err := os.ReadFile("/proc/stat")
	if err != nil {
		return CPUInfo{}, err
	}

	var info CPUInfo
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "cpu ") {
			fields := strings.Fields(line)
			info.User, _ = strconv.ParseFloat(fields[1], 64)
			info.System, _ = strconv.ParseFloat(fields[3], 64)
			info.Idle, _ = strconv.ParseFloat(fields[4], 64)
			info.IOWait, _ = strconv.ParseFloat(fields[5], 64)
			info.Total = info.User + info.System + info.Idle + info.IOWait
			break
		}
	}
	return info, nil
}

// 获取本进程的 CPU 信息（Linux）
func GetProcessCPUInfo(pid int) (ProcessCPUInfo, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return ProcessCPUInfo{}, err
	}

	fields := strings.Fields(string(data))
	userTime, _ := strconv.ParseFloat(fields[13], 64)
	systemTime, _ := strconv.ParseFloat(fields[14], 64)
	totalTime := userTime + systemTime

	return ProcessCPUInfo{
		User:   userTime / sysconfClockTicks(),
		System: systemTime / sysconfClockTicks(),
		Total:  totalTime / sysconfClockTicks(),
	}, nil
}

// 获取系统时钟频率（Linux）
func sysconfClockTicks() float64 {
	// 尝试通过 /proc/stat 推导时钟频率
	ticks, err := calculateClockTicks()
	if err != nil {
		fmt.Println("Warning: Failed to calculate clock ticks. Using default value 100.")
		return 100 // 默认值
	}
	return ticks
}

// 通过 /proc/stat 推导时钟频率
func calculateClockTicks() (float64, error) {
	// 第一次读取 /proc/stat
	firstCPUInfo, err := GetCPUInfo()
	if err != nil {
		return 0, err
	}
	startTime := time.Now()

	// 等待一段时间（例如 100 毫秒）
	time.Sleep(100 * time.Millisecond)

	// 第二次读取 /proc/stat
	secondCPUInfo, err := GetCPUInfo()
	if err != nil {
		return 0, err
	}
	elapsedTime := time.Since(startTime).Seconds()

	// 计算总 CPU 时间增量（以 jiffies 为单位）
	cpuTimeDelta := secondCPUInfo.Total - firstCPUInfo.Total

	// 计算时钟频率
	if elapsedTime == 0 || cpuTimeDelta <= 0 {
		return 0, fmt.Errorf("failed to calculate clock ticks")
	}
	clockTicks := cpuTimeDelta / elapsedTime
	return clockTicks, nil
}
