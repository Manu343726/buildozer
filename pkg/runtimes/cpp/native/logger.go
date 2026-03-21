package native

import (
	"github.com/Manu343726/buildozer/pkg/logging"
)

// Logger returns the logger for the C++ native runtime package
func Logger() *logging.Logger {
	return logging.GetLogger("buildozer.runtime.cpp.native")
}

// ChildLogger returns a child logger for a sub-component
func ChildLogger(componentName string) *logging.Logger {
	return Logger().Child(componentName)
}
