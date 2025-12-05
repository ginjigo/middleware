package middleware

import (
	"bytes"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ginjigo/ginji"
)

func TestLogger(t *testing.T) {
	app := ginji.New()

	// Capture log output
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	app.Use(LoggerWithConfig(LoggerConfig{
		Logger: logger,
	}))

	app.Get("/test", func(c *ginji.Context) error {
		return c.JSON(200, ginji.H{"message": "ok"})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// Verify log was written
	logOutput := buf.String()
	if logOutput == "" {
		t.Error("Expected log output, got empty string")
	}

	// Verify log contains key fields
	if !strings.Contains(logOutput, "status") {
		t.Error("Log missing status field")
	}
	if !strings.Contains(logOutput, "method") {
		t.Error("Log missing method field")
	}
	if !strings.Contains(logOutput, "path") {
		t.Error("Log missing path field")
	}
	if !strings.Contains(logOutput, "latency") {
		t.Error("Log missing latency field")
	}
}

func TestLoggerSkipPaths(t *testing.T) {
	app := ginji.New()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	app.Use(LoggerWithConfig(LoggerConfig{
		Logger:    logger,
		SkipPaths: []string{"/health"},
	}))

	app.Get("/health", func(c *ginji.Context) error {
		return c.Text(200, "OK")
	})

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// Verify no log was written for skipped path
	if buf.Len() > 0 {
		t.Error("Expected no log output for skipped path")
	}
}

func TestLoggerLevels(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		expectedLevel string
		expectedMsg   string
	}{
		{"2xx success", 200, "INFO", "Request processed"},
		{"4xx client error", 404, "WARN", "Client error"},
		{"5xx server error", 500, "ERROR", "Server error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := ginji.New()

			var buf bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))

			app.Use(LoggerWithConfig(LoggerConfig{
				Logger: logger,
			}))

			app.Get("/test", func(c *ginji.Context) error {
				c.Status(tt.statusCode)
				return c.JSON(tt.statusCode, ginji.H{"status": "test"})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			app.ServeHTTP(w, req)

			logOutput := buf.String()
			if !strings.Contains(logOutput, tt.expectedLevel) {
				t.Errorf("Expected log level %s, log: %s", tt.expectedLevel, logOutput)
			}
			if !strings.Contains(logOutput, tt.expectedMsg) {
				t.Errorf("Expected message '%s', log: %s", tt.expectedMsg, logOutput)
			}
		})
	}
}

func TestLoggerSkipFunc(t *testing.T) {
	app := ginji.New()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	app.Use(LoggerWithConfig(LoggerConfig{
		Logger: logger,
		SkipFunc: func(c *ginji.Context) bool {
			return c.Header("X-Skip-Logging") == "true"
		},
	}))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(200, "OK")
	})

	// Request with skip header
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Skip-Logging", "true")
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if buf.Len() > 0 {
		t.Error("Expected no log output when skip function returns true")
	}
}
