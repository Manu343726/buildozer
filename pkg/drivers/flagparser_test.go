package drivers

import (
	"testing"
)

// LogLevel is an example enum type for testing
type LogLevel string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
	WarnLevel  LogLevel = "warn"
	ErrorLevel LogLevel = "error"
)

func (l LogLevel) String() string {
	return string(l)
}

func TestEnumParsing(t *testing.T) {
	fs := NewFlagSet()
	logLevelPtr := Enum(fs, "log-level", InfoLevel, []LogLevel{DebugLevel, InfoLevel, WarnLevel, ErrorLevel}, "Log level")

	tests := []struct {
		name    string
		args    []string
		want    LogLevel
		wantErr bool
	}{
		{
			name:    "space-separated format",
			args:    []string{"--buildozer-log-level", "debug"},
			want:    DebugLevel,
			wantErr: false,
		},
		{
			name:    "equals format",
			args:    []string{"--buildozer-log-level=warn"},
			want:    WarnLevel,
			wantErr: false,
		},
		{
			name:    "default value if not provided",
			args:    []string{"--some-other-flag"},
			want:    InfoLevel,
			wantErr: false,
		},
		{
			name:    "error level",
			args:    []string{"--buildozer-log-level", "error"},
			want:    ErrorLevel,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset the pointer to default
			logLevelPtr = Enum(NewFlagSet(), "log-level", InfoLevel, []LogLevel{DebugLevel, InfoLevel, WarnLevel, ErrorLevel}, "Log level")
			fs := NewFlagSet()
			logLevelPtr = Enum(fs, "log-level", InfoLevel, []LogLevel{DebugLevel, InfoLevel, WarnLevel, ErrorLevel}, "Log level")

			fs.Parse(tt.args)

			if *logLevelPtr != tt.want {
				t.Errorf("got %v, want %v", *logLevelPtr, tt.want)
			}
		})
	}
}

func TestStringParsing(t *testing.T) {
	fs := NewFlagSet()
	namePtr := fs.String("name", "default", "Name flag")

	args := []string{"--buildozer-name", "custom"}
	fs.Parse(args)

	if *namePtr != "custom" {
		t.Errorf("got %v, want %v", *namePtr, "custom")
	}
}

func TestIntParsing(t *testing.T) {
	fs := NewFlagSet()
	maxJobsPtr := fs.Int("max-jobs", 4, "Max jobs")

	args := []string{"--buildozer-max-jobs", "8"}
	fs.Parse(args)

	if *maxJobsPtr != 8 {
		t.Errorf("got %v, want %v", *maxJobsPtr, 8)
	}
}

func TestBoolParsing(t *testing.T) {
	fs := NewFlagSet()
	verbosePtr := fs.Bool("verbose", false, "Verbose")

	args := []string{"--buildozer-verbose", "true"}
	fs.Parse(args)

	if *verbosePtr != true {
		t.Errorf("got %v, want %v", *verbosePtr, true)
	}
}

func TestToolArgPassthrough(t *testing.T) {
	fs := NewFlagSet()
	fs.String("log-level", "info", "Log level")

	args := []string{"--buildozer-log-level", "debug", "-O2", "-Wall", "-c", "main.c", "-o", "main.o"}
	toolArgs := fs.Parse(args)

	expected := []string{"-O2", "-Wall", "-c", "main.c", "-o", "main.o"}
	if len(toolArgs) != len(expected) {
		t.Fatalf("got %d tool args, want %d", len(toolArgs), len(expected))
	}

	for i, arg := range toolArgs {
		if arg != expected[i] {
			t.Errorf("tool arg %d: got %v, want %v", i, arg, expected[i])
		}
	}
}

// ============================================================================
// OPTIONAL FLAG TESTS - Solution 5: Allocate-on-Parse with Pointer-to-Pointer
// ============================================================================
// These tests verify that optional flags return pointer-to-pointer (**T)
// and that we can distinguish "flag not provided" from "flag provided with zero value"

// TestOptionalStringFlagNotProvided verifies optional string flags remain nil when not provided
func TestOptionalStringFlagNotProvided(t *testing.T) {
	fs := NewFlagSet()
	configPtr := fs.StringOpt("config", "Config file path")

	fs.Parse([]string{})

	// configPtr should point to nil
	if *configPtr != nil {
		t.Fatalf("expected *configPtr to be nil, got %v", *configPtr)
	}
}

// TestOptionalStringFlagProvided verifies optional string flags are set when provided
func TestOptionalStringFlagProvided(t *testing.T) {
	fs := NewFlagSet()
	configPtr := fs.StringOpt("config", "Config file path")

	fs.Parse([]string{"--buildozer-config", "myconfig.toml"})

	// configPtr should point to a valid pointer
	if *configPtr == nil {
		t.Fatal("expected *configPtr to be non-nil, got nil")
	}
	if **configPtr != "myconfig.toml" {
		t.Fatalf("expected **configPtr to be 'myconfig.toml', got %q", **configPtr)
	}
}

// TestOptionalStringFlagProvidedWithEqualsFormat verifies equals format works
func TestOptionalStringFlagProvidedWithEqualsFormat(t *testing.T) {
	fs := NewFlagSet()
	configPtr := fs.StringOpt("config", "Config file path")

	fs.Parse([]string{"--buildozer-config=myconfig.toml"})

	if *configPtr == nil {
		t.Fatal("expected *configPtr to be non-nil, got nil")
	}
	if **configPtr != "myconfig.toml" {
		t.Fatalf("expected **configPtr to be 'myconfig.toml', got %q", **configPtr)
	}
}

// TestOptionalIntFlagNotProvided verifies optional int flags remain nil when not provided
func TestOptionalIntFlagNotProvided(t *testing.T) {
	fs := NewFlagSet()
	timeoutPtr := fs.IntOpt("timeout", "Request timeout in seconds")

	fs.Parse([]string{})

	if *timeoutPtr != nil {
		t.Fatalf("expected *timeoutPtr to be nil, got %v", *timeoutPtr)
	}
}

// TestOptionalIntFlagProvided verifies optional int flags are set when provided
func TestOptionalIntFlagProvided(t *testing.T) {
	fs := NewFlagSet()
	timeoutPtr := fs.IntOpt("timeout", "Request timeout in seconds")

	fs.Parse([]string{"--buildozer-timeout", "30"})

	if *timeoutPtr == nil {
		t.Fatal("expected *timeoutPtr to be non-nil, got nil")
	}
	if **timeoutPtr != 30 {
		t.Fatalf("expected **timeoutPtr to be 30, got %d", **timeoutPtr)
	}
}

// TestOptionalIntFlagProvidedWithZeroValue verifies we can distinguish zero from unset
// This is the KEY TEST for solution 5: with pointer-to-pointer storage,
// we can tell if a flag was set to 0 vs. not set at all
func TestOptionalIntFlagProvidedWithZeroValue(t *testing.T) {
	fs := NewFlagSet()
	timeoutPtr := fs.IntOpt("timeout", "Request timeout in seconds")

	fs.Parse([]string{"--buildozer-timeout", "0"})

	// Key test: zero value should be distinguishable from not provided
	if *timeoutPtr == nil {
		t.Fatal("expected *timeoutPtr to be non-nil (zero value provided), got nil")
	}
	if **timeoutPtr != 0 {
		t.Fatalf("expected **timeoutPtr to be 0, got %d", **timeoutPtr)
	}
}

// TestOptionalBoolFlagNotProvided verifies optional bool flags remain nil when not provided
func TestOptionalBoolFlagNotProvided(t *testing.T) {
	fs := NewFlagSet()
	verbosePtr := fs.BoolOpt("verbose", "Verbose output")

	fs.Parse([]string{})

	if *verbosePtr != nil {
		t.Fatalf("expected *verbosePtr to be nil, got %v", *verbosePtr)
	}
}

// TestOptionalBoolFlagProvided verifies optional bool flags are set when provided
func TestOptionalBoolFlagProvided(t *testing.T) {
	fs := NewFlagSet()
	verbosePtr := fs.BoolOpt("verbose", "Verbose output")

	fs.Parse([]string{"--buildozer-verbose", "true"})

	if *verbosePtr == nil {
		t.Fatal("expected *verbosePtr to be non-nil, got nil")
	}
	if **verbosePtr != true {
		t.Fatalf("expected **verbosePtr to be true, got %v", **verbosePtr)
	}
}

// TestOptionalBoolFlagProvidedWithFalse verifies we can distinguish false from unset
// This is the KEY TEST for solution 5: with pointer-to-pointer storage,
// we can tell if a flag was set to false vs. not set at all
func TestOptionalBoolFlagProvidedWithFalse(t *testing.T) {
	fs := NewFlagSet()
	verbosePtr := fs.BoolOpt("verbose", "Verbose output")

	fs.Parse([]string{"--buildozer-verbose", "false"})

	// Key test: false value should be distinguishable from not provided
	if *verbosePtr == nil {
		t.Fatal("expected *verbosePtr to be non-nil (false value provided), got nil")
	}
	if **verbosePtr != false {
		t.Fatalf("expected **verbosePtr to be false, got %v", **verbosePtr)
	}
}

// TestOptionalEnumFlagNotProvided verifies optional enum flags remain nil when not provided
func TestOptionalEnumFlagNotProvided(t *testing.T) {
	fs := NewFlagSet()
	levelPtr := EnumOpt(fs, "level", []LogLevel{InfoLevel, DebugLevel}, "Log level")

	fs.Parse([]string{})

	if *levelPtr != nil {
		t.Fatalf("expected *levelPtr to be nil, got %v", *levelPtr)
	}
}

// TestOptionalEnumFlagProvided verifies optional enum flags are set when provided
func TestOptionalEnumFlagProvided(t *testing.T) {
	fs := NewFlagSet()
	levelPtr := EnumOpt(fs, "level", []LogLevel{InfoLevel, DebugLevel}, "Log level")

	fs.Parse([]string{"--buildozer-level", "debug"})

	if *levelPtr == nil {
		t.Fatal("expected *levelPtr to be non-nil, got nil")
	}
	if **levelPtr != "debug" {
		t.Fatalf("expected **levelPtr to be 'debug', got %q", **levelPtr)
	}
}

// TestOptionalEnumFlagProvidedWithEqualsFormat verifies equals format works for enum flags
func TestOptionalEnumFlagProvidedWithEqualsFormat(t *testing.T) {
	fs := NewFlagSet()
	levelPtr := EnumOpt(fs, "level", []LogLevel{InfoLevel, DebugLevel, WarnLevel}, "Log level")

	fs.Parse([]string{"--buildozer-level=warn"})

	if *levelPtr == nil {
		t.Fatal("expected *levelPtr to be non-nil, got nil")
	}
	if **levelPtr != "warn" {
		t.Fatalf("expected **levelPtr to be 'warn', got %q", **levelPtr)
	}
}

// TestMixedOptionalAndRequired verifies mixing optional and required flags works
func TestMixedOptionalAndRequired(t *testing.T) {
	fs := NewFlagSet()
	logLevelPtr := fs.String("log-level", "info", "Log level")
	configPtr := fs.StringOpt("config", "Config file path")

	fs.Parse([]string{"--buildozer-log-level", "debug"})

	if *logLevelPtr != "debug" {
		t.Fatalf("expected log-level='debug', got %q", *logLevelPtr)
	}
	if *configPtr != nil {
		t.Fatalf("expected config to be nil, got %v", *configPtr)
	}
}

// TestMixedWithBothProvided verifies both optional and required can be provided
func TestMixedWithBothProvided(t *testing.T) {
	fs := NewFlagSet()
	logLevelPtr := fs.String("log-level", "info", "Log level")
	configPtr := fs.StringOpt("config", "Config file path")

	fs.Parse([]string{"--buildozer-log-level", "debug", "--buildozer-config", "/etc/config.toml"})

	if *logLevelPtr != "debug" {
		t.Fatalf("expected log-level='debug', got %q", *logLevelPtr)
	}
	if *configPtr == nil {
		t.Fatal("expected config to be non-nil, got nil")
	}
	if **configPtr != "/etc/config.toml" {
		t.Fatalf("expected config='/etc/config.toml', got %q", **configPtr)
	}
}

// TestOptionalIntFlagWithEqualsFormat verifies equals format works for int flags
func TestOptionalIntFlagWithEqualsFormat(t *testing.T) {
	fs := NewFlagSet()
	timeoutPtr := fs.IntOpt("timeout", "Request timeout in seconds")

	fs.Parse([]string{"--buildozer-timeout=45"})

	if *timeoutPtr == nil {
		t.Fatal("expected *timeoutPtr to be non-nil, got nil")
	}
	if **timeoutPtr != 45 {
		t.Fatalf("expected **timeoutPtr to be 45, got %d", **timeoutPtr)
	}
}

// TestOptionalBoolFlagWithEqualsFormat verifies equals format works for bool flags
func TestOptionalBoolFlagWithEqualsFormat(t *testing.T) {
	fs := NewFlagSet()
	verbosePtr := fs.BoolOpt("verbose", "Verbose output")

	fs.Parse([]string{"--buildozer-verbose=true"})

	if *verbosePtr == nil {
		t.Fatal("expected *verbosePtr to be non-nil, got nil")
	}
	if **verbosePtr != true {
		t.Fatalf("expected **verbosePtr to be true, got %v", **verbosePtr)
	}
}

// TestFlagMetadataIsOptional verifies FlagInfo correctly marks optional flags
func TestFlagMetadataIsOptional(t *testing.T) {
	fs := NewFlagSet()
	fs.String("log-level", "info", "Log level")
	configPtr := fs.StringOpt("config", "Config file path")
	_ = configPtr  // silence unused warning

	flags := fs.Flags()

	if _, ok := flags["log-level"]; !ok {
		t.Fatal("expected 'log-level' flag in map")
	}
	if _, ok := flags["config"]; !ok {
		t.Fatal("expected 'config' flag in map")
	}

	configFlag := flags["config"]
	if !configFlag.IsOptional {
		t.Fatal("expected 'config' to be marked IsOptional=true")
	}

	logLevelFlag := flags["log-level"]
	if logLevelFlag.IsOptional {
		t.Fatal("expected 'log-level' to be marked IsOptional=false")
	}
}

// TestMultipleOptionalFlags verifies multiple optional flags can coexist
func TestMultipleOptionalFlags(t *testing.T) {
	fs := NewFlagSet()
	configPtr := fs.StringOpt("config", "Config file path")
	timeoutPtr := fs.IntOpt("timeout", "Request timeout in seconds")
	verbosePtr := fs.BoolOpt("verbose", "Verbose output")

	fs.Parse([]string{"--buildozer-config", "config.toml", "--buildozer-timeout", "30"})

	// config and timeout were provided, verbose was not
	if *configPtr == nil {
		t.Fatal("expected config to be non-nil")
	}
	if **configPtr != "config.toml" {
		t.Fatalf("expected config='config.toml', got %q", **configPtr)
	}

	if *timeoutPtr == nil {
		t.Fatal("expected timeout to be non-nil")
	}
	if **timeoutPtr != 30 {
		t.Fatalf("expected timeout=30, got %d", **timeoutPtr)
	}

	if *verbosePtr != nil {
		t.Fatalf("expected verbose to be nil, got %v", *verbosePtr)
	}
}
