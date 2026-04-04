package logs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pruvon/pruvon/internal/log"
	"github.com/pruvon/pruvon/internal/middleware"
	"github.com/pruvon/pruvon/internal/models"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func SearchLogs(params models.LogSearchParams) (models.LogSearchResult, error) {
	logFile := "/var/log/pruvon/activity.log"
	result := models.LogSearchResult{
		Page:    params.Page,
		PerPage: params.PerPage,
	}

	// Open log file
	file, err := os.Open(logFile)
	if err != nil {
		return result, err
	}
	defer file.Close()

	// Tüm logları oku ve zamanına göre sırala
	var allLogs []models.ActivityLog
	reader := bufio.NewReader(file)
	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}

		var log models.ActivityLog
		if err := json.Unmarshal([]byte(line), &log); err != nil {
			continue
		}

		// Filtreler
		if params.Username != "" && log.User != params.Username {
			continue
		}
		if params.Query != "" && !strings.Contains(strings.ToLower(log.Action), strings.ToLower(params.Query)) {
			continue
		}

		allLogs = append(allLogs, log)
	}

	// En son loglar başta olacak şekilde sırala
	sort.Slice(allLogs, func(i, j int) bool {
		return allLogs[i].Time.After(allLogs[j].Time)
	})

	// Son 100 log ile sınırla
	if len(allLogs) > 100 {
		allLogs = allLogs[:100]
	}

	// Sayfalama işlemi
	totalLogs := len(allLogs)
	result.TotalLogs = totalLogs

	// Sayfa başına düşen kayıt sayısına göre toplam sayfa sayısını hesapla
	if totalLogs == 0 {
		result.TotalPages = 1
	} else {
		result.TotalPages = (totalLogs + params.PerPage - 1) / params.PerPage
	}

	// Geçerli sayfa numarası kontrolü
	if params.Page < 1 {
		params.Page = 1
	}
	if params.Page > result.TotalPages {
		params.Page = result.TotalPages
	}

	// Sayfa sınırlarını hesapla
	start := (params.Page - 1) * params.PerPage
	end := start + params.PerPage
	if end > totalLogs {
		end = totalLogs
	}

	result.Page = params.Page
	result.Logs = allLogs[start:end]

	return result, nil
}

var logMutex sync.Mutex

// LogActivity logs an activity to the activity log file
func LogActivity(log models.ActivityLog) error {
	logMutex.Lock()
	defer logMutex.Unlock()

	logFile := "/var/log/pruvon/activity.log"

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(logFile), 0755); err != nil {
		return err
	}

	// Open log file in append mode
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Convert log to JSON
	logJson, err := json.Marshal(log)
	if err != nil {
		return err
	}

	// Write log line
	if _, err := f.Write(append(logJson, '\n')); err != nil {
		return err
	}

	return nil
}

// LogWithParams is a helper function to log activities with parameters
func LogWithParams(c *fiber.Ctx, action string, params interface{}) error {
	// Get session info
	sess, _ := middleware.GetStore().Get(c)
	user := sess.Get("user").(string)
	authType := sess.Get("auth_type").(string)

	// Convert params to JSON
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		paramsJSON = []byte("{}")
	}

	return LogActivity(models.ActivityLog{
		Time:       time.Now(),
		RequestID:  uuid.New().String(),
		IP:         c.IP(),
		User:       user,
		AuthType:   authType,
		Action:     action,
		Method:     c.Method(), // Add HTTP method
		Route:      c.Path(),
		Parameters: paramsJSON,
		StatusCode: c.Response().StatusCode(),
	})
}

// GetLogTail reads the last nLines from the file specified by filePath
// using the external `tail` command.
func GetLogTail(filePath string, nLines int) ([]string, error) {
	if nLines <= 0 {
		return []string{}, nil
	}

	// Check if file exists and is not empty first
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.LogWarning(fmt.Sprintf("[GetLogTail] File does not exist: %s", filePath))
			return []string{}, nil // Return empty slice if file doesn't exist
		}
		return nil, fmt.Errorf("failed to get file info for %s: %w", filePath, err)
	}

	if fileInfo.Size() == 0 {
		log.LogInfo(fmt.Sprintf("[GetLogTail] File is empty: %s", filePath))
		return []string{}, nil // Return empty slice if file is empty
	}

	// Use the `tail` command to get the last nLines
	cmd := exec.Command("tail", "-n", strconv.Itoa(nLines), filePath)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	log.LogInfo(fmt.Sprintf("[GetLogTail] Running command: %s", cmd.String()))
	err = cmd.Run()
	if err != nil {
		errMsg := fmt.Sprintf("failed to run tail command for %s: %v. Stderr: %s", filePath, err, stderr.String())
		log.LogError(errMsg)
		return nil, errors.New(errMsg)
	}

	// Split the output into lines
	output := strings.TrimSpace(stdout.String())
	if output == "" {
		return []string{}, nil // No output from tail (might happen with very small files/nLines)
	}
	lines := strings.Split(output, "\n")

	log.LogInfo(fmt.Sprintf("[GetLogTail] Successfully read %d lines using tail from %s", len(lines), filePath))
	return lines, nil
}
