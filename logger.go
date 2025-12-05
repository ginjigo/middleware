package middleware

import (
	"log/slog"
	"time"

	"github.com/ginjigo/ginji"
)

// LoggerConfig defines configuration for the logger middleware.
type LoggerConfig struct {
	// Logger is the slog logger instance to use. If nil, uses engine's logger.
	Logger *slog.Logger

	// SkipPaths is a list of paths to skip logging (e.g., health checks).
	SkipPaths []string

	// SkipFunc allows custom logic to skip logging for certain requests.
	SkipFunc func(*ginji.Context) bool
}

// DefaultLoggerConfig returns the default logger configuration.
func DefaultLoggerConfig() LoggerConfig {
	return LoggerConfig{
		SkipPaths: []string{},
	}
}

// Logger returns a middleware that logs HTTP requests using structured logging.
func Logger() ginji.Middleware {
	return LoggerWithConfig(DefaultLoggerConfig())
}

// LoggerWithConfig returns a middleware with custom logger configuration.
func LoggerWithConfig(config LoggerConfig) ginji.Middleware {
	skipPaths := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipPaths[path] = true
	}

	return func(c *ginji.Context) error {
		// Skip logging if path is in skip list
		if skipPaths[c.Req.URL.Path] {
			return c.Next()
		}

		// Skip logging if skip function returns true
		if config.SkipFunc != nil && config.SkipFunc(c) {
			return c.Next()
		}

		start := time.Now()
		path := c.Req.URL.Path
		query := c.Req.URL.RawQuery

		// Process request
		err := c.Next() // Call next middleware/handler

		// Calculate latency
		latency := time.Since(start)

		// Determine which logger to use
		logger := config.Logger
		if logger == nil {
			// Use engine's logger if available
			if c.Req.Context().Value("engine") != nil {
				if engine, ok := c.Req.Context().Value("engine").(*ginji.Engine); ok {
					logger = engine.Logger
				}
			}
		}

		// Fallback to default slog if no logger configured
		if logger == nil {
			logger = slog.Default()
		}

		// Build log attributes
		attrs := []slog.Attr{
			slog.Int("status", c.StatusCode()),
			slog.String("method", c.Req.Method),
			slog.String("path", path),
			slog.String("ip", c.Req.RemoteAddr),
			slog.Duration("latency", latency),
			slog.String("user_agent", c.Header("User-Agent")),
		}

		if query != "" {
			attrs = append(attrs, slog.String("query", query))
		}

		// Add error if present
		if c.IsAborted() {
			attrs = append(attrs, slog.Bool("aborted", true))
		}

		// Log at appropriate level based on status code
		statusCode := c.StatusCode()
		level := slog.LevelInfo
		message := "Request processed"

		if statusCode >= 500 {
			level = slog.LevelError
			message = "Server error"
		} else if statusCode >= 400 {
			level = slog.LevelWarn
			message = "Client error"
		}

		logger.LogAttrs(c.Req.Context(), level, message, attrs...)
		return err
	}
}
