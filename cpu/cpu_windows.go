//go:build windows

package cpu

// GetCPUInfo returns CPU information for Windows
func GetCPUInfo() (CPUInfo, error) {
	return CPUInfo{
		User:   0,
		System: 0,
		Idle:   0,
		Total:  0,
	}, nil
}

// GetProcessCPUInfo returns process CPU information for Windows
func GetProcessCPUInfo(pid int) (ProcessCPUInfo, error) {
	return ProcessCPUInfo{
		User:   0,
		System: 0,
		Total:  0,
	}, nil
}
