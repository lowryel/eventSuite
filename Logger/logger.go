package logger

import (
	"fmt"
	"runtime"
)

var (
	// LogsPath is the path the log files will be created
	LogsPath string
)

// DevLog logs information for the developer, during development to
func DevLog(msg interface{}) {
	_, file, line, _ := runtime.Caller(1)
	fmt.Printf("File: %s, Line: %d - %s\n", file, line, msg)
}
