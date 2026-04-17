package remote

import "github.com/Manu343726/buildozer/pkg/logging"

// Log returns the logger for the remote runtimes package
func Log() *logging.Logger {
	return logging.Log("runtimes").Child("remote")
}
