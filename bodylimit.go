package middleware

import (
	"fmt"
	"io"
	"net/http"

	"github.com/ginjigo/ginji"
)

// BodyLimitConfig defines the configuration for body limit middleware.
type BodyLimitConfig struct {
	// MaxBytes is the maximum allowed size of the request body in bytes.
	MaxBytes int64

	// ErrorMessage is the custom error message to return when limit is exceeded.
	// If empty, a default message will be used.
	ErrorMessage string

	// StatusCode is the HTTP status code to return when limit is exceeded.
	// Defaults to 413 (Request Entity Too Large).
	StatusCode int
}

// DefaultBodyLimitConfig returns a default configuration with 4MB limit.
func DefaultBodyLimitConfig() BodyLimitConfig {
	return BodyLimitConfig{
		MaxBytes:     4 << 20, // 4 MB
		ErrorMessage: "",
		StatusCode:   http.StatusRequestEntityTooLarge,
	}
}

// BodyLimit returns a middleware that limits the size of request bodies.
// Usage:
//
//	app.Use(middleware.BodyLimit(10 << 20)) // 10 MB limit
func BodyLimit(maxBytes int64) ginji.Middleware {
	config := DefaultBodyLimitConfig()
	config.MaxBytes = maxBytes
	return BodyLimitWithConfig(config)
}

// BodyLimitWithConfig returns a middleware with custom configuration.
func BodyLimitWithConfig(config BodyLimitConfig) ginji.Middleware {
	// Set defaults
	if config.MaxBytes <= 0 {
		config.MaxBytes = DefaultBodyLimitConfig().MaxBytes
	}
	if config.StatusCode == 0 {
		config.StatusCode = http.StatusRequestEntityTooLarge
	}
	if config.ErrorMessage == "" {
		config.ErrorMessage = fmt.Sprintf("Request body too large. Maximum allowed size is %d bytes", config.MaxBytes)
	}

	return func(c *ginji.Context) error {
		// Check Content-Length header first (if present)
		if c.Req.ContentLength > config.MaxBytes {
			c.AbortWithStatusJSON(config.StatusCode, ginji.H{
				"error":    config.ErrorMessage,
				"maxBytes": config.MaxBytes,
				"received": c.Req.ContentLength,
			})
			return nil
		}

		// Wrap the request body with a limited reader
		if c.Req.Body != nil {
			c.Req.Body = &limitedReadCloser{
				ReadCloser: c.Req.Body,
				limit:      config.MaxBytes,
				read:       0,
				config:     &config,
				context:    c,
			}
		}

		return c.Next()
	}
}

// limitedReadCloser wraps an io.ReadCloser and enforces a size limit.
type limitedReadCloser struct {
	io.ReadCloser
	limit   int64
	read    int64
	config  *BodyLimitConfig
	context *ginji.Context
}

// Read reads from the underlying reader while enforcing the limit.
func (l *limitedReadCloser) Read(p []byte) (n int, err error) {
	n, err = l.ReadCloser.Read(p)
	l.read += int64(n)

	if l.read > l.limit {
		return n, fmt.Errorf("request body size exceeds limit of %d bytes", l.limit)
	}

	return n, err
}

// Helper functions for common size limits

// BodyLimit1MB returns middleware with 1MB limit.
func BodyLimit1MB() ginji.Middleware {
	return BodyLimit(1 << 20)
}

// BodyLimit5MB returns middleware with 5MB limit.
func BodyLimit5MB() ginji.Middleware {
	return BodyLimit(5 << 20)
}

// BodyLimit10MB returns middleware with 10MB limit.
func BodyLimit10MB() ginji.Middleware {
	return BodyLimit(10 << 20)
}

// BodyLimit50MB returns middleware with 50MB limit.
func BodyLimit50MB() ginji.Middleware {
	return BodyLimit(50 << 20)
}
