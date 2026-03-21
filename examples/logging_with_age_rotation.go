// Package example demonstrates age-based log file rotation using buildozer's logging system.
package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Manu343726/buildozer/pkg/logging"
	"github.com/Manu343726/buildozer/pkg/logging/sinks"
)

func main() {
	// Example 1: Using FileSink directly with age-based rotation
	// Rotate logs when they exceed 10MB OR are older than 7 days
	handler, err := sinks.FileSink(sinks.FileSinkConfig{
		Path:       "/tmp/app.log",
		MaxSizeB:   10 * 1024 * 1024, // 10MB
		MaxFiles:   10,
		MaxAgeDays: 7, // Keep logs for 7 days
		JSONFormat: false,
		HandlerOpts: &slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	})
	if err != nil {
		panic(err)
	}

	logger := slog.New(handler)
	logger.Info("Application started", slog.Time("timestamp", time.Now()))

	// Example 2: Using helper functions with age-based rotation
	// Create a JSON log file that rotates at 50MB or after 14 days
	jsonHandler, err := sinks.JSONFileSink("/tmp/app-json.log", 50, 14)
	if err != nil {
		panic(err)
	}
	jsonLogger := slog.New(jsonHandler)
	jsonLogger.Info("JSON logger initialized", slog.String("format", "json"))

	// Example 3: Using logging package with YAML configuration
	// The configuration can specify maxAgeDays for file sinks:
	// sinks:
	//   - name: archive
	//     type: file
	//     level: info
	//     path: /var/log/app/archive.log
	//     max_size_b: 104857600  # 100MB
	//     max_files: 30
	//     max_age_days: 90        # Keep for 90 days
	//
	// This configuration would automatically rotate logs when they:
	// - Exceed 100MB, OR
	// - Are older than 90 days
	// - Keep at most 30 backup files

	// Example 4: Enable file sink programmatically with age-based rotation
	loggingConfig := logging.DefaultLoggingConfig()
	registry := logging.NewRegistry()
	factory := logging.NewFactory(registry)

	// Add a custom file sink with 7-day age limit
	if err := factory.InitializeFromConfig(loggingConfig); err != nil {
		panic(err)
	}

	sinkConfig := logging.SinkConfig{
		Name:       "aged-log",
		Type:       "file",
		Level:      "debug",
		Path:       filepath.Join(os.TempDir(), "aged.log"),
		MaxSizeB:   50 * 1024 * 1024, // 50MB
		MaxFiles:   5,
		MaxAgeDays: 7, // Rotate after 7 days
		JSONFormat: false,
	}

	sink, err := factory.CreateSink(sinkConfig)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()
	logger.InfoContext(ctx, "File sink with age-based rotation configured",
		slog.String("sink", sink.Name),
		slog.Int("max_age_days", 7),
	)

	// Example 5: No age limit (traditional size-based rotation only)
	noAgeHandler, err := sinks.TextFileSink("/tmp/no-age-limit.log", 100, 0)
	if err != nil {
		panic(err)
	}
	noAgeLogger := slog.New(noAgeHandler)
	noAgeLogger.Info("This log will rotate by size only (age-based rotation disabled)")
}
