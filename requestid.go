package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/ginjigo/ginji"
)

// RequestIDConfig defines configuration for request ID middleware.
type RequestIDConfig struct {
	// Generator is a function that generates unique request IDs.
	// If nil, a default UUID-like generator is used.
	Generator func() string

	// RequestIDHeader is the header name for the request ID.
	// Default: "X-Request-ID"
	RequestIDHeader string

	// ResponseIDHeader is the header name for the response ID.
	// Default: "X-Request-ID"
	ResponseIDHeader string

	// ContextKey is the key to store the request ID in context.
	// Default: "request_id"
	ContextKey string
}

// DefaultRequestIDConfig returns default request ID configuration.
func DefaultRequestIDConfig() RequestIDConfig {
	return RequestIDConfig{
		Generator:        generateUUID,
		RequestIDHeader:  "X-Request-ID",
		ResponseIDHeader: "X-Request-ID",
		ContextKey:       "request_id",
	}
}

// RequestID returns a request ID middleware with default configuration.
func RequestID() ginji.Middleware {
	return RequestIDWithConfig(DefaultRequestIDConfig())
}

// RequestIDWithConfig returns a request ID middleware with custom configuration.
func RequestIDWithConfig(config RequestIDConfig) ginji.Middleware {
	// Set defaults
	if config.Generator == nil {
		config.Generator = generateUUID
	}
	if config.RequestIDHeader == "" {
		config.RequestIDHeader = "X-Request-ID"
	}
	if config.ResponseIDHeader == "" {
		config.ResponseIDHeader = "X-Request-ID"
	}
	if config.ContextKey == "" {
		config.ContextKey = "request_id"
	}

	return func(c *ginji.Context) error {
		// Check if request already has an ID
		requestID := c.Header(config.RequestIDHeader)
		if requestID == "" {
			// Generate new ID
			requestID = config.Generator()
		}

		// Store in context
		c.Set(config.ContextKey, requestID)

		// Add to response header
		c.SetHeader(config.ResponseIDHeader, requestID)

		return c.Next()
	}
}

// generateUUID generates a UUID-like random identifier.
func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate UUID: %v", err))
	}

	// Format as UUID v4 (xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant is 10

	return fmt.Sprintf("%s-%s-%s-%s-%s",
		hex.EncodeToString(b[0:4]),
		hex.EncodeToString(b[4:6]),
		hex.EncodeToString(b[6:8]),
		hex.EncodeToString(b[8:10]),
		hex.EncodeToString(b[10:16]),
	)
}

// GetRequestID is a helper to get the request ID from context.
func GetRequestID(c *ginji.Context) string {
	return c.GetString("request_id")
}
