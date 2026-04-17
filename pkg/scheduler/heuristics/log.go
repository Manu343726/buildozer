package heuristics

import (
	"github.com/Manu343726/buildozer/pkg/logging"
)

// Log creates a logger for the heuristics package
func Log() *logging.Logger {
	return logging.Log("heuristics")
}

// LogSubsystem creates a logger for the heuristics with a subsystem
func LogSubsystem(subsystem string) *logging.Logger {
	return logging.Log("heuristics").Child(subsystem)
}
