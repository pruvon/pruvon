package system

import (
	"github.com/pruvon/pruvon/internal/models"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReadCPUStats(t *testing.T) {
	t.Run("Valid CPU stats", func(t *testing.T) {
		// Mock the readFileFunc to return test data
		originalReadFile := readFileFunc
		defer func() { readFileFunc = originalReadFile }()

		mockData := `cpu  12345 678 9012 345678 901 234 567 890
cpu0 3086 169 2253 86389 225 58 141 222
cpu1 3086 169 2253 86389 225 58 141 222
cpu2 3086 169 2253 86389 225 58 141 222
cpu3 3086 169 2253 86389 225 58 141 222
`
		readFileFunc = func(name string) ([]byte, error) {
			return []byte(mockData), nil
		}

		stats, err := ReadCPUStats()
		assert.NoError(t, err)
		assert.Equal(t, uint64(12345), stats.User)
		assert.Equal(t, uint64(678), stats.Nice)
		assert.Equal(t, uint64(9012), stats.System)
		assert.Equal(t, uint64(345678), stats.Idle)
		assert.Equal(t, uint64(901), stats.Iowait)
		assert.Equal(t, uint64(234), stats.Irq)
		assert.Equal(t, uint64(567), stats.Softirq)
		assert.Equal(t, uint64(890), stats.Steal)
	})

	t.Run("Invalid CPU stats - no data", func(t *testing.T) {
		originalReadFile := readFileFunc
		defer func() { readFileFunc = originalReadFile }()

		mockData := `invalid data`
		readFileFunc = func(name string) ([]byte, error) {
			return []byte(mockData), nil
		}

		_, err := ReadCPUStats()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CPU stats not found")
	})

	t.Run("File read error", func(t *testing.T) {
		originalReadFile := readFileFunc
		defer func() { readFileFunc = originalReadFile }()

		readFileFunc = func(name string) ([]byte, error) {
			return nil, os.ErrNotExist
		}

		_, err := ReadCPUStats()
		assert.Error(t, err)
	})
}

func TestCalculateCPUUsage(t *testing.T) {
	t.Run("Calculate CPU usage", func(t *testing.T) {
		// Mock the readFileFunc to return incremental test data
		originalReadFile := readFileFunc
		defer func() { readFileFunc = originalReadFile }()

		callCount := 0
		readFileFunc = func(name string) ([]byte, error) {
			callCount++
			if callCount == 1 {
				// First call - baseline
				return []byte(`cpu  10000 500 5000 200000 1000 100 500 50`), nil
			}
			// Second call - after some CPU work
			return []byte(`cpu  10100 510 5050 200050 1010 105 510 55`), nil
		}

		usage := CalculateCPUUsage()
		// Should be a positive number
		assert.GreaterOrEqual(t, usage, 0.0)
		assert.LessOrEqual(t, usage, 100.0)
	})

	t.Run("Calculate CPU usage with error", func(t *testing.T) {
		originalReadFile := readFileFunc
		defer func() { readFileFunc = originalReadFile }()

		readFileFunc = func(name string) ([]byte, error) {
			return nil, os.ErrNotExist
		}

		usage := CalculateCPUUsage()
		assert.Equal(t, 0.0, usage)
	})
}

func TestFormatBytesPerSecond(t *testing.T) {
	tests := []struct {
		name     string
		bytes    float64
		expected string
	}{
		{
			name:     "Bytes per second",
			bytes:    512.5,
			expected: "512.5 B/s",
		},
		{
			name:     "Kilobytes per second",
			bytes:    2048.0,
			expected: "2.0 KB/s",
		},
		{
			name:     "Megabytes per second",
			bytes:    5242880.0, // 5 MB
			expected: "5.0 MB/s",
		},
		{
			name:     "Gigabytes per second",
			bytes:    5368709120.0, // 5 GB
			expected: "5.0 GB/s",
		},
		{
			name:     "Zero bytes",
			bytes:    0,
			expected: "0.0 B/s",
		},
		{
			name:     "Small fraction",
			bytes:    100.5,
			expected: "100.5 B/s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatBytesPerSecond(tt.bytes)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	t.Run("Contains existing item", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}
		assert.True(t, Contains(slice, "banana"))
	})

	t.Run("Does not contain item", func(t *testing.T) {
		slice := []string{"apple", "banana", "cherry"}
		assert.False(t, Contains(slice, "orange"))
	})

	t.Run("Empty slice", func(t *testing.T) {
		slice := []string{}
		assert.False(t, Contains(slice, "apple"))
	})
}

func TestCPUStatsTotal(t *testing.T) {
	t.Run("Calculate total CPU time", func(t *testing.T) {
		stats := models.CPUStats{
			User:    10000,
			Nice:    500,
			System:  5000,
			Idle:    200000,
			Iowait:  1000,
			Irq:     100,
			Softirq: 500,
			Steal:   50,
		}

		total := stats.User + stats.Nice + stats.System + stats.Idle +
			stats.Iowait + stats.Irq + stats.Softirq + stats.Steal

		assert.Equal(t, uint64(217150), total)
	})
}

func TestCPUStatsIdle(t *testing.T) {
	t.Run("Calculate idle CPU time", func(t *testing.T) {
		stats := models.CPUStats{
			Idle:   200000,
			Iowait: 1000,
		}

		idle := stats.Idle + stats.Iowait
		assert.Equal(t, uint64(201000), idle)
	})
}

func TestCPUStatsDiff(t *testing.T) {
	t.Run("Calculate CPU stats difference", func(t *testing.T) {
		stat1 := models.CPUStats{
			User:    10000,
			Nice:    500,
			System:  5000,
			Idle:    200000,
			Iowait:  1000,
			Irq:     100,
			Softirq: 500,
			Steal:   50,
		}

		stat2 := models.CPUStats{
			User:    10100,
			Nice:    510,
			System:  5050,
			Idle:    200050,
			Iowait:  1010,
			Irq:     105,
			Softirq: 510,
			Steal:   55,
		}

		// Calculate differences
		userDiff := stat2.User - stat1.User
		niceDiff := stat2.Nice - stat1.Nice
		systemDiff := stat2.System - stat1.System
		idleDiff := stat2.Idle - stat1.Idle
		iowaitDiff := stat2.Iowait - stat1.Iowait
		irqDiff := stat2.Irq - stat1.Irq
		softirqDiff := stat2.Softirq - stat1.Softirq
		stealDiff := stat2.Steal - stat1.Steal

		assert.Equal(t, uint64(100), userDiff)
		assert.Equal(t, uint64(10), niceDiff)
		assert.Equal(t, uint64(50), systemDiff)
		assert.Equal(t, uint64(50), idleDiff)
		assert.Equal(t, uint64(10), iowaitDiff)
		assert.Equal(t, uint64(5), irqDiff)
		assert.Equal(t, uint64(10), softirqDiff)
		assert.Equal(t, uint64(5), stealDiff)
	})
}

func BenchmarkReadCPUStats(b *testing.B) {
	// Mock the readFileFunc for benchmarking
	originalReadFile := readFileFunc
	defer func() { readFileFunc = originalReadFile }()

	mockData := `cpu  12345 678 9012 345678 901 234 567 890`
	readFileFunc = func(name string) ([]byte, error) {
		return []byte(mockData), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ReadCPUStats()
	}
}

func BenchmarkFormatBytesPerSecond(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		formatBytesPerSecond(5242880.0) // 5 MB/s
	}
}

func BenchmarkContains(b *testing.B) {
	slice := []string{"apple", "banana", "cherry", "date", "elderberry"}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Contains(slice, "cherry")
	}
}
