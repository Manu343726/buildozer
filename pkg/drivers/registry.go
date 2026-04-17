package drivers

import (
	"fmt"
	"sync"

	"github.com/Manu343726/buildozer/pkg/driver"
)

// driverRegistry maps driver names to their implementations.
// Protected by driverRegistryMu for thread-safe registration.
var (
	driverRegistry   = make(map[string]driver.Driver)
	driverRegistryMu sync.RWMutex
)

// RegisterDriver registers a driver implementation in the global registry.
// This is called by each driver package during initialization.
func RegisterDriver(name string, d driver.Driver) {
	driverRegistryMu.Lock()
	defer driverRegistryMu.Unlock()
	driverRegistry[name] = d
}

// GetDriver returns the driver implementation for the given driver name.
// Returns nil if the driver is not registered.
func GetDriver(driverName string) driver.Driver {
	driverRegistryMu.RLock()
	defer driverRegistryMu.RUnlock()
	return driverRegistry[driverName]
}

// ConstructRuntimeIDFromConfig constructs a runtime ID using the driver's
// ConstructRuntimeID method. Returns error if driver not found or if required
// config fields are missing.
func ConstructRuntimeIDFromConfig(driverName string, cfgMap map[string]interface{}) (string, error) {
	d := GetDriver(driverName)
	if d == nil {
		return "", fmt.Errorf("driver %q not found", driverName)
	}
	return d.ConstructRuntimeID(cfgMap)
}
