package ar

import "github.com/Manu343726/buildozer/pkg/logging"

// Log returns the logger for the ar driver package
func Log() *logging.Logger {
	return logging.Log().Child("drivers").Child("ar")
}
