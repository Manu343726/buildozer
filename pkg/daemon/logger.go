package daemon

import "github.com/Manu343726/buildozer/pkg/logging"

// Log returns the logger for the daemon package
// If daemonID is provided, it will be added to all logs from this logger
func Log(daemonID ...string) *logging.Logger {
	if len(daemonID) > 0 && daemonID[0] != "" {
		return logging.Log(daemonID[0]).Child("daemon")
	}
	return logging.Log().Child("daemon")
}
