package logs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pruvon/pruvon/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchLogs(t *testing.T) {
	// Create a temporary log file for testing
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "activity.log")

	// Create test log entries
	testTime1, _ := time.Parse(time.RFC3339, "2023-01-01T10:00:00Z")
	testTime2, _ := time.Parse(time.RFC3339, "2023-01-01T11:00:00Z")
	testLogs := []models.ActivityLog{
		{
			Time:       testTime1,
			RequestID:  "req1",
			IP:         "127.0.0.1",
			User:       "testuser",
			AuthType:   "github",
			Action:     "login",
			Method:     "POST",
			Route:      "/api/login",
			Parameters: []byte("{}"),
			StatusCode: 200,
		},
		{
			Time:       testTime2,
			RequestID:  "req2",
			IP:         "127.0.0.1",
			User:       "testuser",
			AuthType:   "github",
			Action:     "app_create",
			Method:     "POST",
			Route:      "/api/apps",
			Parameters: []byte("{\"name\":\"testapp\"}"),
			StatusCode: 201,
		},
	}

	// Write test logs to file
	file, err := os.Create(logFile)
	require.NoError(t, err)
	defer file.Close()

	for _, log := range testLogs {
		logJSON, err := json.Marshal(log)
		require.NoError(t, err)
		file.Write(append(logJSON, '\n'))
	}

	// Note: SearchLogs uses hardcoded path, so we skip this test for now
	// In production, we'd refactor to accept log file path as parameter
	t.Skip("SearchLogs uses hardcoded path, needs refactoring for testability")
}

func TestLogActivity(t *testing.T) {
	// Note: LogActivity uses hardcoded path, so we skip this test for now
	// In production, we'd refactor to accept log file path as parameter
	t.Skip("LogActivity uses hardcoded path, needs refactoring for testability")
}

func TestGetLogTail(t *testing.T) {
	// Create a temporary log file
	tempDir := t.TempDir()
	logFile := filepath.Join(tempDir, "test.log")

	// Create test content
	testLines := []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	}

	file, err := os.Create(logFile)
	require.NoError(t, err)
	defer file.Close()

	for _, line := range testLines {
		file.WriteString(line + "\n")
	}

	// Test getting last 3 lines
	lines, err := GetLogTail(logFile, 3)
	assert.NoError(t, err)
	assert.Len(t, lines, 3)
	assert.Equal(t, "line 3", lines[0])
	assert.Equal(t, "line 4", lines[1])
	assert.Equal(t, "line 5", lines[2])

	// Test getting more lines than available
	lines, err = GetLogTail(logFile, 10)
	assert.NoError(t, err)
	assert.Len(t, lines, 5)

	// Test with zero lines
	lines, err = GetLogTail(logFile, 0)
	assert.NoError(t, err)
	assert.Len(t, lines, 0)

	// Test with non-existent file
	lines, err = GetLogTail("/non/existent/file.log", 5)
	assert.NoError(t, err) // Should return empty slice for non-existent file
	assert.Len(t, lines, 0)
}
