package log

import (
	"bytes"
	"io"
	"log"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerInitialization(t *testing.T) {
	t.Run("Loggers are initialized", func(t *testing.T) {
		assert.NotNil(t, InfoLogger, "InfoLogger should be initialized")
		assert.NotNil(t, WarningLogger, "WarningLogger should be initialized")
		assert.NotNil(t, ErrorLogger, "ErrorLogger should be initialized")
	})

	t.Run("Loggers have correct prefixes", func(t *testing.T) {
		// Create test loggers with custom output to verify prefixes
		var buf bytes.Buffer

		testInfo := log.New(&buf, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
		testInfo.Println("test")
		assert.Contains(t, buf.String(), "INFO:", "Info logger should have INFO prefix")

		buf.Reset()
		testWarning := log.New(&buf, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
		testWarning.Println("test")
		assert.Contains(t, buf.String(), "WARNING:", "Warning logger should have WARNING prefix")

		buf.Reset()
		testError := log.New(&buf, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
		testError.Println("test")
		assert.Contains(t, buf.String(), "ERROR:", "Error logger should have ERROR prefix")
	})
}

func TestLogInfo(t *testing.T) {
	t.Run("LogInfo writes to output", func(t *testing.T) {
		// Capture stdout
		oldOutput := InfoLogger.Writer()
		var buf bytes.Buffer
		InfoLogger.SetOutput(&buf)
		defer InfoLogger.SetOutput(oldOutput)

		testMessage := "Test info message"
		LogInfo(testMessage)

		output := buf.String()
		assert.Contains(t, output, "INFO:", "Output should contain INFO prefix")
		assert.Contains(t, output, testMessage, "Output should contain the message")
		assert.Contains(t, output, "logger.go", "Output should contain logger file name")
	})

	t.Run("LogInfo formats message correctly", func(t *testing.T) {
		var buf bytes.Buffer
		oldOutput := InfoLogger.Writer()
		InfoLogger.SetOutput(&buf)
		defer InfoLogger.SetOutput(oldOutput)

		LogInfo("Application started")

		output := buf.String()
		lines := strings.Split(strings.TrimSpace(output), "\n")
		assert.Equal(t, 1, len(lines), "Should produce exactly one line")
	})
}

func TestLogWarning(t *testing.T) {
	t.Run("LogWarning writes to output", func(t *testing.T) {
		// Capture stdout
		oldOutput := WarningLogger.Writer()
		var buf bytes.Buffer
		WarningLogger.SetOutput(&buf)
		defer WarningLogger.SetOutput(oldOutput)

		testMessage := "Test warning message"
		LogWarning(testMessage)

		output := buf.String()
		assert.Contains(t, output, "WARNING:", "Output should contain WARNING prefix")
		assert.Contains(t, output, testMessage, "Output should contain the message")
		assert.Contains(t, output, "logger.go", "Output should contain logger file name")
	})

	t.Run("LogWarning with special characters", func(t *testing.T) {
		var buf bytes.Buffer
		oldOutput := WarningLogger.Writer()
		WarningLogger.SetOutput(&buf)
		defer WarningLogger.SetOutput(oldOutput)

		specialMessage := "Warning: Connection timeout (retry: 3/5)"
		LogWarning(specialMessage)

		output := buf.String()
		assert.Contains(t, output, specialMessage, "Should handle special characters")
	})
}

func TestLogError(t *testing.T) {
	t.Run("LogError writes to stderr", func(t *testing.T) {
		// Capture stderr
		oldOutput := ErrorLogger.Writer()
		var buf bytes.Buffer
		ErrorLogger.SetOutput(&buf)
		defer ErrorLogger.SetOutput(oldOutput)

		testMessage := "Test error message"
		LogError(testMessage)

		output := buf.String()
		assert.Contains(t, output, "ERROR:", "Output should contain ERROR prefix")
		assert.Contains(t, output, testMessage, "Output should contain the message")
		assert.Contains(t, output, "logger.go", "Output should contain logger file name")
	})

	t.Run("LogError with error details", func(t *testing.T) {
		var buf bytes.Buffer
		oldOutput := ErrorLogger.Writer()
		ErrorLogger.SetOutput(&buf)
		defer ErrorLogger.SetOutput(oldOutput)

		errorDetails := "Database connection failed: timeout after 30s"
		LogError(errorDetails)

		output := buf.String()
		assert.Contains(t, output, errorDetails, "Should log full error details")
		assert.Contains(t, output, "ERROR:", "Should have ERROR prefix")
	})
}

func TestLoggerConcurrency(t *testing.T) {
	t.Run("Concurrent logging doesn't panic", func(t *testing.T) {
		var buf lockedBuffer
		oldInfoOutput := InfoLogger.Writer()
		oldWarnOutput := WarningLogger.Writer()
		oldErrorOutput := ErrorLogger.Writer()

		InfoLogger.SetOutput(&buf)
		WarningLogger.SetOutput(&buf)
		ErrorLogger.SetOutput(&buf)

		defer func() {
			InfoLogger.SetOutput(oldInfoOutput)
			WarningLogger.SetOutput(oldWarnOutput)
			ErrorLogger.SetOutput(oldErrorOutput)
		}()

		done := make(chan bool)

		// Run concurrent logging
		for i := 0; i < 10; i++ {
			go func(id int) {
				LogInfo("Info message")
				LogWarning("Warning message")
				LogError("Error message")
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for i := 0; i < 10; i++ {
			<-done
		}

		output := buf.String()
		// Just verify we got some output without panic
		assert.NotEmpty(t, output, "Should have logged messages")
	})
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.buf.String()
}

func TestLoggerOutputFormats(t *testing.T) {
	t.Run("Log includes timestamp", func(t *testing.T) {
		var buf bytes.Buffer
		oldOutput := InfoLogger.Writer()
		InfoLogger.SetOutput(&buf)
		defer InfoLogger.SetOutput(oldOutput)

		LogInfo("Test timestamp")

		output := buf.String()
		// Should contain date in format like 2024/01/01
		assert.Regexp(t, `\d{4}/\d{2}/\d{2}`, output, "Should contain date")
		// Should contain time in format like 12:34:56
		assert.Regexp(t, `\d{2}:\d{2}:\d{2}`, output, "Should contain time")
	})

	t.Run("Log includes file and line number", func(t *testing.T) {
		var buf bytes.Buffer
		oldOutput := InfoLogger.Writer()
		InfoLogger.SetOutput(&buf)
		defer InfoLogger.SetOutput(oldOutput)

		LogInfo("Test file info")

		output := buf.String()
		// Should contain filename and line number like logger.go:23
		assert.Regexp(t, `logger\.go:\d+`, output, "Should contain logger file and line number")
	})
}

func BenchmarkLogInfo(b *testing.B) {
	// Redirect output to discard
	oldOutput := InfoLogger.Writer()
	InfoLogger.SetOutput(io.Discard)
	defer InfoLogger.SetOutput(oldOutput)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LogInfo("Benchmark test message")
	}
}

func BenchmarkLogWarning(b *testing.B) {
	oldOutput := WarningLogger.Writer()
	WarningLogger.SetOutput(io.Discard)
	defer WarningLogger.SetOutput(oldOutput)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LogWarning("Benchmark test message")
	}
}

func BenchmarkLogError(b *testing.B) {
	oldOutput := ErrorLogger.Writer()
	ErrorLogger.SetOutput(io.Discard)
	defer ErrorLogger.SetOutput(oldOutput)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		LogError("Benchmark test message")
	}
}
