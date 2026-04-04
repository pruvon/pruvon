package log

import (
	"log"
	"os"
)

var (
	WarningLogger *log.Logger
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
)

func init() {
	// Initialize the loggers
	InfoLogger = log.New(os.Stdout, "INFO: ", log.Ldate|log.Ltime|log.Lshortfile)
	WarningLogger = log.New(os.Stdout, "WARNING: ", log.Ldate|log.Ltime|log.Lshortfile)
	ErrorLogger = log.New(os.Stderr, "ERROR: ", log.Ldate|log.Ltime|log.Lshortfile)
}

// LogInfo logs an informational message
func LogInfo(message string) {
	InfoLogger.Println(message)
}

// LogWarning logs a warning message
func LogWarning(message string) {
	WarningLogger.Println(message)
}

// LogError logs an error message
func LogError(message string) {
	ErrorLogger.Println(message)
}
