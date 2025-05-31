//go:build linux

package cpu

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// TODO: cgroup +cgroup v2 的cpu占用率

// https://man7.org/linux/man-pages/man5/proc_pid_stat.5.html
var clocksPerSec = float64(100)

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
		User:   userTime,
		System: systemTime,
		Total:  totalTime,
	}, nil
}

// 获取系统时钟频率（Linux）
func sysconfClockTicks() float64 {
	return clocksPerSec
}
