package cpu

import (
	"os"
	"runtime"
	"testing"
	"time"
)

// TestCPUInfoAccuracyDarwin tests the accuracy of CPU info by comparing multiple readings (Darwin/macOS)
func TestCPUInfoAccuracyDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping test on non-Darwin platform")
	}
	// Get multiple CPU readings to verify consistency
	readings := make([]CPUInfo, 5)
	for i := 0; i < 5; i++ {
		info, err := GetCPUInfo()
		if err != nil {
			t.Fatalf("Failed to get CPU info: %v", err)
		}
		readings[i] = info
		time.Sleep(100 * time.Millisecond)
	}

	// Verify that Total is correctly calculated as the sum of components (User + System + Idle)
	for i, info := range readings {
		calculatedTotal := info.User + info.System + info.Idle
		if info.Total != calculatedTotal {
			t.Errorf("Reading %d: Total (%.2f) does not match sum of components (%.2f)",
				i, info.Total, calculatedTotal)
		}
	}

	// Verify that CPU values are increasing over time (or at least not decreasing)
	for i := 1; i < len(readings); i++ {
		if readings[i].Total < readings[i-1].Total {
			t.Errorf("Total CPU time decreased between readings %d and %d", i-1, i)
		}
	}
}

// TestProcessCPUInfoDarwin tests getting process CPU info on Darwin/macOS
func TestProcessCPUInfoDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping test on non-Darwin platform")
	}
	// Get two readings and check that values are reasonable
	proc1, err := GetProcessCPUInfo(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to get process CPU info: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	proc2, err := GetProcessCPUInfo(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to get process CPU info: %v", err)
	}
	if proc2.Total < proc1.Total {
		t.Errorf("Process total CPU time did not increase: before=%.2f after=%.2f", proc1.Total, proc2.Total)
	}
}
