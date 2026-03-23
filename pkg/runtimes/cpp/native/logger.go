package native

import (
	"github.com/Manu343726/buildozer/pkg/logging"
)

// Log returns the logger for the C++ native runtime package
func Log() *logging.Logger {
	return logging.Log().Child("runtimes").Child("cpp").Child("native")
}
