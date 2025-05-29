package utils

import (
	"log"
	"os"
)

// showDebug controls debug output (set by config)
var showDebug = false

// SetDebug enables or disables debug logging
func SetDebug(enabled bool) {
	showDebug = enabled
}

// LogDebug logs debug info (only if debug is enabled)
func LogDebug(format string, v ...interface{}) {
	if !showDebug {
		return
	}
	log.Printf("[DEBUG] "+format, v...)
}

// LogInfo logs general information
func LogInfo(format string, v ...interface{}) {
	log.Printf("[INFO] "+format, v...)
}

// LogWarn logs warnings (non-critical issues)
func LogWarn(format string, v ...interface{}) {
	log.Printf("[WARN] "+format, v...)
}

// LogError logs errors
func LogError(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}

// LogFatal logs error and exits
func LogFatal(format string, v ...interface{}) {
	log.Printf("[FATAL] "+format, v...)
	os.Exit(1)
}
