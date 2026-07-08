package logger

import (
	"fmt"
	"os"
	"sync"
)

var (
	enabled bool
	once    sync.Once
)

// init checks DEBUG environment variable once
func init() {
	once.Do(func() {
		debug := os.Getenv("DEBUG")
		enabled = debug == "true" || debug == "1" || debug == "yes"
	})
}

// Enabled returns true if DEBUG logging is enabled
func Enabled() bool {
	return enabled
}

// Debug prints a debug message if DEBUG is enabled
func Debug(args ...interface{}) {
	if enabled {
		fmt.Println(args...)
	}
}

// Debugf prints a formatted debug message if DEBUG is enabled
func Debugf(format string, args ...interface{}) {
	if enabled {
		fmt.Printf(format+"\n", args...)
	}
}

// Info prints an info message if DEBUG is enabled
func Info(args ...interface{}) {
	if enabled {
		fmt.Println(args...)
	}
}

// Infof prints a formatted info message if DEBUG is enabled
func Infof(format string, args ...interface{}) {
	if enabled {
		fmt.Printf(format+"\n", args...)
	}
}

// Error prints an error message if DEBUG is enabled
func Error(args ...interface{}) {
	if enabled {
		fmt.Println(args...)
	}
}

// Errorf prints a formatted error message if DEBUG is enabled
func Errorf(format string, args ...interface{}) {
	if enabled {
		fmt.Printf(format+"\n", args...)
	}
}

// Warn prints a warning message if DEBUG is enabled
func Warn(args ...interface{}) {
	if enabled {
		fmt.Println(args...)
	}
}

// Warnf prints a formatted warning message if DEBUG is enabled
func Warnf(format string, args ...interface{}) {
	if enabled {
		fmt.Printf(format+"\n", args...)
	}
}
