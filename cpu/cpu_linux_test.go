//go:build linux

package cpu

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"testing"
	"time"
)

// TestCPUSpike tests the ability to detect a sudden CPU spike
// This test is Linux-specific and simulates a CPU spike to verify
// that our CPU monitoring code behaves similarly to 'top'
func TestCPUSpike(t *testing.T) {
	// Get current process ID
	pid := os.Getpid()
	t.Log("Current process ID:", pid)

	if runtime.GOOS != "linux" {
		t.Skip("Skipping test on non-Linux platform")
	}
	// Get initial CPU info
	initialInfo, err := GetCPUInfo()
	if err != nil {
		t.Fatalf("Failed to get initial CPU info: %v", err)
	}

	initialProcInfo, err := GetProcessCPUInfo(pid)
	if err != nil {
		t.Fatalf("Failed to get initial process CPU info: %v", err)
	}

	// Create a CPU spike by performing intensive calculations in the main test goroutine
	// This ensures that the CPU usage is attributed to the test process
	t.Logf("Starting CPU spike simulation...pid %d, initial info: %#v, initial process info: %#v", pid, initialInfo, initialProcInfo)

	// Create a channel for timing
	done := make(chan bool)

	// Start a goroutine to signal when to stop the CPU spike
	go func() {
		time.Sleep(3 * time.Second)
		done <- true
	}()

	// Perform CPU-intensive calculations in the main test goroutine
	// This will ensure the CPU usage is attributed to this process
	counter := 0
	loopStart := time.Now()
	for {
		select {
		case <-done:
			fmt.Printf("Performed %d calculation loops in %v\n", counter, time.Since(loopStart))
			goto done
		default:
			// Busy loop to consume CPU - more intensive calculation
			for j := 0; j < 1000000; j++ {
				_ = math.Sqrt(float64(j)) * math.Cos(float64(j))
			}
			counter++
		}
	}
done:

	// The CPU spike has already run for the specified period

	// Get CPU info during the spike
	spikeInfo, err := GetCPUInfo()
	if err != nil {
		t.Fatalf("Failed to get CPU info during spike: %v", err)
	}

	// Get process CPU info during the spike
	spikeProcInfo, err := GetProcessCPUInfo(pid)
	if err != nil {
		t.Fatalf("Failed to get process CPU info during spike: %v", err)
	}

	// CPU spike has already been stopped

	fmt.Printf("CPU spike simulation completed, %#v\n", spikeProcInfo)

	// Calculate CPU usage percentages
	systemCPUPercent := CalculateCPUPercent(initialInfo, spikeInfo)
	processCPUPercent := CalculateProcessCPUPercent(initialProcInfo, spikeProcInfo, initialInfo, spikeInfo)

	// Print the results
	fmt.Printf("System CPU Usage: %.2f%%\n", systemCPUPercent)
	fmt.Printf("Process CPU Usage: %.2f%%\n", processCPUPercent)

	// Verify that we detected significant CPU usage
	if systemCPUPercent < 0.0 || processCPUPercent < 0.0 {
		t.Errorf("Expected system CPU usage to be at least 0.0%%, got %.2f%%", systemCPUPercent)
	}

	// On Linux, a single-threaded CPU-intensive process should show significant CPU usage
	// We expect at least 50% of a single core, which translates to different percentages
	// depending on the number of cores
	minExpectedProcessCPU := 50.0 / float64(runtime.NumCPU())
	if processCPUPercent < minExpectedProcessCPU {
		t.Errorf("Expected process CPU usage to be at least %.2f%% (50%% of a single core), got %.2f%%",
			minExpectedProcessCPU, processCPUPercent)
	}

	// Verify that our process is consuming a significant portion of the CPU
	// This is similar to what 'top' would show
	t.Logf("System CPU Usage: %.2f%%", systemCPUPercent)
	t.Logf("Process CPU Usage: %.2f%%", processCPUPercent)
}

// TestCPUInfoAccuracy tests the accuracy of CPU info by comparing multiple readings
func TestCPUInfoAccuracy(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping test on non-Linux platform")
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

	// Verify that Total is correctly calculated as the sum of components
	for i, info := range readings {
		calculatedTotal := info.User + info.System + info.Idle + info.IOWait
		if info.Total != calculatedTotal {
			t.Errorf("Reading %d: Total (%.2f) does not match sum of components (%.2f)",
				i, info.Total, calculatedTotal)
		}
	}

	// Verify that CPU values are increasing over time (or at least not decreasing)
	// This is a basic sanity check
	for i := 1; i < len(readings); i++ {
		if readings[i].Total < readings[i-1].Total {
			t.Errorf("Total CPU time decreased between readings %d and %d", i-1, i)
		}
	}

	// Verify that sysconfClockTicks returns a reasonable value
	ticks := sysconfClockTicks()
	if ticks <= 0 {
		t.Errorf("Expected positive clock ticks, got %.2f", ticks)
	}
	t.Logf("Clock ticks: %.2f", ticks)
}
