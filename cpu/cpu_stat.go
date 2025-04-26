package cpu

// CPUInfo 存储 CPU 时间信息
type CPUInfo struct {
	User   float64
	System float64
	Idle   float64
	IOWait float64
	Total  float64
}

// ProcessCPUInfo 存储进程的 CPU 时间信息
type ProcessCPUInfo struct {
	User   float64
	System float64
	Total  float64
}

// 计算全局 CPU 使用率
func CalculateCPUPercent(prev, curr CPUInfo) float64 {
	totalDelta := curr.Total - prev.Total
	idleDelta := curr.Idle - prev.Idle
	return (totalDelta - idleDelta) / totalDelta * 100
}

// 计算本进程的 CPU 使用率
func CalculateProcessCPUPercent(prevProc, currProc ProcessCPUInfo, prevSys, currSys CPUInfo) float64 {
	totalDelta := currSys.Total - prevSys.Total
	procDelta := currProc.Total - prevProc.Total
	return (procDelta / totalDelta) * 100
}
