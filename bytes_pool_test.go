package pulse

import (
	"testing"
)

func TestGetBytes(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		expected int
	}{
		{"small size", 100, 1024},                     // 1KB
		{"medium size", 2000, 2048},                   // 2KB
		{"large size", 5000, 5120},                    // 5KB
		{"very large size", 1024 * 1024, 1024 * 1024}, // 1MB
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := getBytes(tt.size)
			if buf == nil {
				t.Fatal("getBytes returned nil")
			}
			if cap(*buf) < tt.expected {
				t.Errorf("getBytes(%d) = %d, want >= %d", tt.size, cap(*buf), tt.expected)
			}
			putBytes(buf)
		})
	}
}

func TestPutBytes(t *testing.T) {
	// Test putting back a small buffer
	smallBuf := getBytes(100)
	putBytes(smallBuf)

	// Test putting back a large buffer
	largeBuf := getBytes(1024 * 1024)
	putBytes(largeBuf)

	// Test putting back a nil buffer
	putBytes(nil)

	// Test putting back an empty buffer
	emptyBuf := &[]byte{}
	putBytes(emptyBuf)
}

func TestGetBigBytes(t *testing.T) {
	tests := []struct {
		name     string
		size     int
		expected int
	}{
		{"512KB", 512 * 1024, 512 * 1024},
		{"1MB", 1024 * 1024, 1024 * 1024},
		{"2MB", 2 * 1024 * 1024, 2 * 1024 * 1024},
		{"32MB", 32 * 1024 * 1024, 32 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := getBigBytes(tt.size)
			if buf == nil {
				t.Fatal("getBigBytes returned nil")
			}
			if cap(*buf) < tt.expected {
				t.Errorf("getBigBytes(%d) = %d, want >= %d", tt.size, cap(*buf), tt.expected)
			}
			putBigBytes(buf)
		})
	}
}

func TestPutBigBytes(t *testing.T) {
	// Test putting back a big buffer
	bigBuf := getBigBytes(1024 * 1024)
	putBigBytes(bigBuf)

	// Test putting back a nil buffer
	putBigBytes(nil)

	// Test putting back an empty buffer
	emptyBuf := &[]byte{}
	putBigBytes(emptyBuf)
}

func TestBytesPoolReuse(t *testing.T) {
	// Test that we can reuse the same buffer multiple times
	buf1 := getBytes(100)
	putBytes(buf1)
	buf2 := getBytes(100)
	putBytes(buf2)

	// Test that we can reuse the same big buffer multiple times
	bigBuf1 := getBigBytes(1024 * 1024)
	putBigBytes(bigBuf1)
	bigBuf2 := getBigBytes(1024 * 1024)
	putBigBytes(bigBuf2)
}
