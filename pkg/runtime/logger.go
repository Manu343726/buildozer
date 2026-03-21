package runtime

import "github.com/Manu343726/buildozer/pkg/logging"

// Logger returns the logger for the runtime package
func Logger() *logging.Logger {
	return logging.GetLogger("buildozer.runtime")
}
