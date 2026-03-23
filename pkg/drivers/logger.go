package drivers

import "github.com/Manu343726/buildozer/pkg/logging"

// Log returns the logger for the drivers package
func Log() *logging.Logger {
	return logging.Log().Child("drivers")
}
