package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/ginjigo/ginji"
)

// bufferedResponseWriter buffers the response until we know if timeout occurred
type bufferedResponseWriter struct {
	header http.Header
	buf    *bytes.Buffer
	status int
}

func newBufferedResponseWriter() *bufferedResponseWriter {
	return &bufferedResponseWriter{
		header: make(http.Header),
		buf:    new(bytes.Buffer),
		status: 200,
	}
}

func (w *bufferedResponseWriter) Header() http.Header {
	return w.header
}

func (w *bufferedResponseWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *bufferedResponseWriter) WriteHeader(statusCode int) {
	w.status = statusCode
}

// copyTo copies the buffered response to the actual response writer
func (w *bufferedResponseWriter) copyTo(dst http.ResponseWriter) {
	// Copy headers
	for k, v := range w.header {
		for _, vv := range v {
			dst.Header().Add(k, vv)
		}
	}
	// Write status
	dst.WriteHeader(w.status)
	// Write body
	_, _ = dst.Write(w.buf.Bytes())
}

// TimeoutConfig defines the configuration for timeout middleware.
type TimeoutConfig struct {
	// Timeout is the duration before the request times out.
	Timeout time.Duration

	// ErrorMessage is the message returned when a timeout occurs.
	ErrorMessage string

	// StatusCode is the HTTP status code for timeout responses.
	// Default: 408 Request Timeout or 504 Gateway Timeout
	StatusCode int

	// SkipFunc allows skipping timeout for certain requests.
	SkipFunc func(*ginji.Context) bool
}

// DefaultTimeoutConfig returns default timeout configuration.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		Timeout:      30 * time.Second,
		ErrorMessage: "Request timeout",
		StatusCode:   ginji.StatusGatewayTimeout,
	}
}

// Timeout returns middleware that enforces a timeout on requests.
func Timeout(duration time.Duration) ginji.Middleware {
	config := DefaultTimeoutConfig()
	config.Timeout = duration
	return TimeoutWithConfig(config)
}

// TimeoutWithConfig returns middleware with custom timeout configuration.
func TimeoutWithConfig(config TimeoutConfig) ginji.Middleware {
	if config.Timeout <= 0 {
		config.Timeout = 30 * time.Second
	}
	if config.StatusCode == 0 {
		config.StatusCode = ginji.StatusGatewayTimeout
	}
	if config.ErrorMessage == "" {
		config.ErrorMessage = "Request timeout"
	}

	return func(c *ginji.Context) error {
		// Skip if skip function returns true
		if config.SkipFunc != nil && config.SkipFunc(c) {
			return c.Next()
		}

		// Create a context with timeout
		ctx, cancel := context.WithTimeout(c.Req.Context(), config.Timeout)
		defer cancel()

		// Replace request context
		c.Req = c.Req.WithContext(ctx)

		// Replace response writer with buffered version
		originalRes := c.Res
		buffered := newBufferedResponseWriter()
		c.Res = buffered

		// Create a deep copy of the context for the goroutine
		// This is crucial because:
		// 1. The original context might be returned to the pool if timeout occurs
		// 2. The goroutine might still be running and accessing context data
		// 3. Maps (Keys, Params) are shared in shallow copy, causing race conditions
		cp := c.DeepCopy()
		// The copy has its own maps but shares handlers and services (read-only in handlers)
		// cp.index is currently pointing to this middleware.
		// cp.Next() will increment it and call the next handler.

		// Channel to signal completion
		done := make(chan struct{})

		// Run handler in goroutine
		go func() {
			defer func() {
				// Recover from any panics in the handler goroutine
				// We can't propagate panics since we're in a goroutine
				// The timeout will handle the response, we just prevent the crash
				// With deep copy, panic recovery is safe from race conditions
				_ = recover()
			}()

			cp.Next()
			close(done)
		}()

		// Wait for either completion or timeout
		select {
		case <-done:
			// Handler completed successfully - write buffered response
			// Restore original writer first? No, we copy to it.
			c.Res = originalRes
			buffered.copyTo(originalRes)

			// We need to sync the context state back if needed?
			// e.g. if handlers modified c.Keys, cp.Keys is modified (map is ref).
			// But c.index is NOT modified by cp.Next().
			// c.index is still at Timeout middleware.
			// When Timeout returns, c.Next() (in ServeHTTP or prev mw) continues?
			// No, c.Next() iterates.
			// If Timeout was called by c.Next(), then c.Next() loop continues.
			// But we already ran the rest of the chain in the goroutine!
			// We DO NOT want c.Next() to run the rest of the chain AGAIN.

			// So we must advance c.index to the end.
			c.Abort() // This sets index to abort index, preventing further execution in current chain.
			return nil

		case <-ctx.Done():
			// Timeout occurred
			c.Res = originalRes // Restore original writer

			// DO NOT restore c.Res - let handler continue writing to buffer which will be discarded
			// Wait, we just restored it.
			// The goroutine uses cp.Res which is buffered. So it's fine.

			if ctx.Err() == context.DeadlineExceeded {
				// Write directly to original writer
				c.Res.Header().Set("Content-Type", "application/json")
				c.Res.WriteHeader(config.StatusCode)
				jsonData, _ := json.Marshal(ginji.H{
					"error":   config.ErrorMessage,
					"timeout": config.Timeout.String(),
				})
				_, _ = c.Res.Write(jsonData)
			}

			// Abort the chain so we don't continue
			c.Abort()
			return nil
		}
	}
}

// TimeoutSeconds returns middleware with timeout in seconds.
func TimeoutSeconds(seconds int) ginji.Middleware {
	return Timeout(time.Duration(seconds) * time.Second)
}

// TimeoutMinutes returns middleware with timeout in minutes.
func TimeoutMinutes(minutes int) ginji.Middleware {
	return Timeout(time.Duration(minutes) * time.Minute)
}
