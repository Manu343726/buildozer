// Package drivers provides shared driver infrastructure and utilities.
//
// The FlagSet type manages typed driver flags with a pflag-like API:
//
//	fs := NewFlagSet()
//	// Flags with defaults:
//	logLevel := fs.String("log-level", "info", "Log level")
//	maxJobs := fs.Int("max-jobs", 4, "Max concurrent jobs")
//	verbose := fs.Bool("verbose", false, "Verbose output")
//
//	// Optional flags (nil if not provided):
//	configPath := fs.StringOpt("config", "Config file path")
//	timeout := fs.IntOpt("timeout", "Request timeout in seconds")
//	mode := EnumOpt(fs, "mode", []Mode{Debug, Release}, "Build mode")
//
//	toolArgs := fs.Parse(args)
//
//	fmt.Println(*logLevel)               // dereference pointer to get value
//	if *configPath != nil {
//		fmt.Println(**configPath)        // dereference pointer-to-pointer to get value
//	}
//	if *timeout != nil {
//		fmt.Println(**timeout)           // dereference pointer-to-pointer to get value
//	}
//
// Recognized flag formats:
//
//	--buildozer-flag <value>      (space-separated)
//	--buildozer-flag=<value>      (equals format)
//
// Optional flags return pointer-to-pointer initially pointing to nil.
// After Parse(), if the flag was provided, the inner pointer will be non-nil,
// otherwise it remains nil. This allows distinguishing "flag not provided"
// from "flag provided with zero value" (e.g., --buildozer-timeout 0).
// For Enum flags, the string value is matched exactly against the String() representation of each
// valid enum value (case-sensitive).
package drivers

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// FlagInfo holds metadata about a registered flag
type FlagInfo struct {
	Name        string                      // Flag name without --buildozer- prefix
	Description string                      // Help text
	Value       interface{}                 // pointer to the flag value
	Parser      func(string) (any, error)   // Function to parse flag value from string
	IsOptional  bool                        // True if this is an optional flag (no default)
	IsBool      bool                        // True if this is a boolean flag (can be used without value)
}

// FlagSet manages typed driver flags and provides parsing similar to pflag.
// Flags are registered with typed methods (String, Int, Bool, StringSlice)
// which return pointers to the parsed values. Enum flags use a standalone generic function.
type FlagSet struct {
	flags map[string]*FlagInfo
}

// NewFlagSet creates a new empty FlagSet
func NewFlagSet() *FlagSet {
	return &FlagSet{
		flags: make(map[string]*FlagInfo),
	}
}

// String registers a string flag with the given name, default value, and description.
// Returns a pointer to the parsed value.
func (fs *FlagSet) String(name string, defaultVal string, description string) *string {
	ptr := new(string)
	*ptr = defaultVal

	fs.flags[name] = &FlagInfo{
		Name:        name,
		Description: description,
		Value:       ptr,
		Parser: func(s string) (any, error) {
			return s, nil
		},
	}
	return ptr
}

// StringOpt registers an optional string flag with the given name and description.
// Returns a pointer-to-pointer to the flag value (initially pointing to nil).
// After Parse(), if the flag was provided, the inner pointer will be non-nil.
// Usage:
//   configPtr := fs.StringOpt("config", "Config file path")
//   fs.Parse(args)
//   if *configPtr != nil {
//       fmt.Println(**configPtr)  // dereference to get the actual string
//   }
func (fs *FlagSet) StringOpt(name string, description string) **string {
	ptr := new(*string)  // ptr is **string, *ptr is nil initially

	fs.flags[name] = &FlagInfo{
		Name:        name,
		Description: description,
		Value:       ptr,  // Store pointer-to-pointer
		IsOptional:  true,
		Parser: func(s string) (any, error) {
			return s, nil
		},
	}
	return ptr  // Return pointer-to-pointer so Parse() updates are visible
}

// Int registers an int flag with the given name, default value, and description.
// Returns a pointer to the parsed value.
func (fs *FlagSet) Int(name string, defaultVal int, description string) *int {
	ptr := new(int)
	*ptr = defaultVal

	fs.flags[name] = &FlagInfo{
		Parser: func(s string) (any, error) {
			intVal, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("invalid int: %w", err)
			}
			return intVal, nil
		},
		Name:        name,
		Description: description,
		Value:       ptr,
	}
	return ptr
}

// IntOpt registers an optional int flag with the given name and description.
// Returns a pointer-to-pointer to the flag value (initially pointing to nil).
// After Parse(), if the flag was provided, the inner pointer will be non-nil.
// Usage:
//   timeoutPtr := fs.IntOpt("timeout", "Request timeout in seconds")
//   fs.Parse(args)
//   if *timeoutPtr != nil {
//       fmt.Println(**timeoutPtr)  // dereference to get the actual int
//   }
func (fs *FlagSet) IntOpt(name string, description string) **int {
	ptr := new(*int)  // ptr is **int, *ptr is nil initially

	fs.flags[name] = &FlagInfo{
		Parser: func(s string) (any, error) {
			intVal, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("invalid int: %w", err)
			}
			return intVal, nil
		},
		Name:        name,
		Description: description,
		Value:       ptr,  // Store pointer-to-pointer
		IsOptional:  true,
	}
	return ptr  // Return pointer-to-pointer so Parse() updates are visible
}

// Bool registers a bool flag with the given name, default value, and description.
// Returns a pointer to the parsed value.
func (fs *FlagSet) Bool(name string, defaultVal bool, description string) *bool {
	ptr := new(bool)
	*ptr = defaultVal

	fs.flags[name] = &FlagInfo{
		Name:        name,
		Description: description,
		Value:       ptr,
		IsBool:      true,
		Parser: func(s string) (any, error) {
			boolVal := s == "true" || s == "1" || s == "yes" || s == ""
			return boolVal, nil
		},
	}
	return ptr
}

// BoolOpt registers an optional bool flag with the given name and description.
// Returns a pointer-to-pointer to the flag value (initially pointing to nil).
// After Parse(), if the flag was provided, the inner pointer will be non-nil.
// Usage:
//   verbosePtr := fs.BoolOpt("verbose", "Verbose output")
//   fs.Parse(args)
//   if *verbosePtr != nil {
//       fmt.Println(**verbosePtr)  // dereference to get the actual bool
//   }
func (fs *FlagSet) BoolOpt(name string, description string) **bool {
	ptr := new(*bool)  // ptr is **bool, *ptr is nil initially

	fs.flags[name] = &FlagInfo{
		Name:        name,
		Description: description,
		Value:       ptr,  // Store pointer-to-pointer
		IsOptional:  true,
		Parser: func(s string) (any, error) {
			boolVal := s == "true" || s == "1" || s == "yes"
			return boolVal, nil
		},
	}
	return ptr  // Return pointer-to-pointer so Parse() updates are visible
}

// Enum registers an enum flag with the given name, default value, valid enum values, and description.
// The enum type T must implement fmt.Stringer interface (String() method).
// Valid enum values are converted to strings using String() for validation.
// Returns a pointer to the parsed value.
// This is a standalone generic function (not a method) to work with Go's type parameter rules.
//
// Example:
//
//	type LogLevel string
//	func (l LogLevel) String() string { return string(l) }
//
//	const (
//		DebugLevel LogLevel = "debug"
//		InfoLevel  LogLevel = "info"
//		WarnLevel  LogLevel = "warn"
//	)
//
//	fs := NewFlagSet()
//	levelPtr := Enum(fs, "log-level", InfoLevel, []LogLevel{DebugLevel, InfoLevel, WarnLevel}, "Log level")
//	fs.Parse(args)
//	fmt.Println(*levelPtr)  // e.g., "info"
func Enum[T fmt.Stringer](fs *FlagSet, name string, defaultVal T, validValues []T, description string) *T {
	ptr := new(T)
	*ptr = defaultVal

	// Create a map from string representation to enum value for fast lookup
	enumMap := make(map[string]T)
	for _, v := range validValues {
		enumMap[v.String()] = v
	}

	fs.flags[name] = &FlagInfo{
		Name:        name,
		Description: description,
		Value:       ptr,
		Parser: func(s string) (any, error) {
			if val, ok := enumMap[s]; ok {
				return val, nil
			}
			validStrs := make([]string, 0, len(validValues))
			for _, v := range validValues {
				validStrs = append(validStrs, v.String())
			}
			return nil, fmt.Errorf("invalid enum value: %s (valid values: %v)", s, validStrs)
		},
	}
	return ptr
}

// EnumOpt registers an optional enum flag with the given name, valid enum values, and description.
// The enum type T must implement fmt.Stringer interface (String() method).
// Valid enum values are converted to strings using String() for validation.
// Returns a pointer-to-pointer to the flag value (initially pointing to nil).
// After Parse(), if the flag was provided, the inner pointer will be non-nil.
// This is a standalone generic function (not a method) to work with Go's type parameter rules.
//
// Example:
//
//	type Mode string
//	func (m Mode) String() string { return string(m) }
//
//	const (
//		DebugMode Mode = "debug"
//		ReleaseMode Mode = "release"
//	)
//
//	fs := NewFlagSet()
//	modePtr := EnumOpt(fs, "mode", []Mode{DebugMode, ReleaseMode}, "Build mode")
//	fs.Parse(args)
//	if *modePtr != nil {
//		fmt.Println(**modePtr)  // dereference to get the actual enum value
//	}
func EnumOpt[T fmt.Stringer](fs *FlagSet, name string, validValues []T, description string) **T {
	ptr := new(*T)  // ptr is **T, *ptr is nil initially

	// Create a map from string representation to enum value for fast lookup
	enumMap := make(map[string]T)
	for _, v := range validValues {
		enumMap[v.String()] = v
	}

	fs.flags[name] = &FlagInfo{
		Name:        name,
		Description: description,
		Value:       ptr,  // Store pointer-to-pointer
		IsOptional:  true,
		Parser: func(s string) (any, error) {
			if val, ok := enumMap[s]; ok {
				return val, nil
			}
			validStrs := make([]string, 0, len(validValues))
			for _, v := range validValues {
				validStrs = append(validStrs, v.String())
			}
			return nil, fmt.Errorf("invalid enum value: %s (valid values: %v)", s, validStrs)
		},
	}
	return ptr  // Return pointer-to-pointer so Parse() updates are visible
}

// Parse extracts buildozer-prefixed flags from command line arguments.
// Handles both --buildozer-flag <value> and --buildozer-flag=<value> syntax.
// For boolean flags, also handles --buildozer-flag without a value (sets to true).
// Returns remaining arguments (tool flags).
// The registered flag pointers are updated with parsed values.
func (fs *FlagSet) Parse(args []string) []string {
	var toolArgs []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		matched := false

		// Check each registered flag
		for flagName, flagInfo := range fs.flags {
			flagPrefix := "--buildozer-" + flagName

			var val string
			var found bool

			if arg == flagPrefix {
				// Check if this is a boolean flag (can be used without value)
				if flagInfo.IsBool {
					// Boolean flag without value - treat as "true"
					val = "true"
					found = true
				} else if i+1 < len(args) {
					// --buildozer-flag <value>
					i++ // skip next arg
					val = args[i]
					found = true
				}
			} else if strings.HasPrefix(arg, flagPrefix+"=") {
				// --buildozer-flag=<value>
				val = arg[len(flagPrefix+"="):]
				found = true
			}

			if found {
				// Parse the value using the parser function
				parsedVal, err := flagInfo.Parser(val)
				if err != nil {
					// Note: In a real implementation, you might want to return an error
					// or log it. For now, we'll silently skip invalid values.
					continue
				}

				// Set the parsed value using reflection
				ptrVal := reflect.ValueOf(flagInfo.Value)
				
				if flagInfo.IsOptional {
					// For optional flags, Value is a pointer-to-pointer (**T)
					// Dereference once to get the inner pointer (*T)
					innerPtrVal := ptrVal.Elem()
					
					// Create a new pointer to hold the parsed value
					newPtr := reflect.New(reflect.TypeOf(parsedVal))
					newPtr.Elem().Set(reflect.ValueOf(parsedVal))
					
					// Assign the new pointer through the pointer-to-pointer
					innerPtrVal.Set(newPtr)
				} else {
					// For required flags, Value is a pointer (*T)
					if ptrVal.Kind() == reflect.Ptr && !ptrVal.IsNil() {
						elemVal := ptrVal.Elem()
						elemVal.Set(reflect.ValueOf(parsedVal))
					}
				}

				matched = true
				break
			}
		}

		if !matched {
			// All other args are tool-specific
			toolArgs = append(toolArgs, arg)
		}
	}

	return toolArgs
}

// Flags returns metadata about all registered flags in this FlagSet
func (fs *FlagSet) Flags() map[string]*FlagInfo {
	return fs.flags
}

// Exposes standard driver flags:
//   --buildozer-log-level (default: warn)
//   --buildozer-config (default: empty)
//   --buildozer-runtime (default: empty, use config value)
//   --buildozer-list-runtimes (default: false)
//   --buildozer-daemon-host (default: localhost)
//   --buildozer-daemon-port (default: 6789)
//   --buildozer-standalone (default: false, runs in-process daemon)
var (
	StandardDriverFlags = NewFlagSet()
	LogLevelPtr         = StandardDriverFlags.String("log-level", "warn", "Log level: debug, info, warn, error")
	ConfigPathPtr       = StandardDriverFlags.String("config", "", "Explicit path to .buildozer config file")
	RuntimePtr          = StandardDriverFlags.String("runtime", "", "Initial runtime ID (overrides config)")
	ListRuntimesPtr     = StandardDriverFlags.Bool("list-runtimes", false, "List available runtimes compatible with this driver and exit")
	StandalonePtr       = StandardDriverFlags.Bool("standalone", false, "Run in standalone mode (in-process daemon)")
	DaemonHostPtr       = StandardDriverFlags.StringOpt("daemon-host", "Buildozer daemon host (default: localhost)")
	DaemonPortPtr       = StandardDriverFlags.IntOpt("daemon-port", "Buildozer daemon port (default: 6789)")
)
