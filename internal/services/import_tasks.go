package services

import (
	"sync"
	"time"
)

// ImportTaskDetails holds information about an ongoing service import task.
type ImportTaskDetails struct {
	SvcType          string
	SvcName          string
	TempFilePath     string
	OriginalFilename string
	User             string
	AuthType         string
	StartTime        time.Time
}

var (
	// ImportTasks stores details of ongoing import tasks, keyed by task ID.
	ImportTasks = make(map[string]ImportTaskDetails)
	// ImportTasksMutex protects access to the ImportTasks map.
	ImportTasksMutex sync.Mutex
)
