package ar_common

import "github.com/Manu343726/buildozer/pkg/logging"

// Log returns the logger for the ar_common package
func Log() *logging.Logger {
	return logging.Log().Child("drivers").Child("ar_common")
}
