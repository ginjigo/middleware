package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ginjigo/ginji"
)

// RateLimiterConfig defines the configuration for rate limiter middleware.
type RateLimiterConfig struct {
	// Max is the maximum number of requests allowed in the time window.
	Max int

	// Window is the time window for rate limiting.
	Window time.Duration

	// KeyFunc is a function that returns the key for rate limiting.
	// Common keys: IP address, user ID, API key.
	// Default: uses client IP
	KeyFunc func(*ginji.Context) string

	// ErrorMessage is returned when rate limit is exceeded.
	ErrorMessage string

	// StatusCode is the HTTP status code when rate limit is exceeded.
	// Default: 429 Too Many Requests
	StatusCode int

	// SkipFunc allows skipping rate limiting for certain requests.
	SkipFunc func(*ginji.Context) bool

	// Headers determines whether to add rate limit headers to the response.
	Headers bool

	// TrustedProxies is a list of trusted proxy IP addresses.
	// If empty, X-Forwarded-For headers are not trusted.
	TrustedProxies []string
}

// bucket represents a token bucket for rate limiting.
type bucket struct {
	tokens    int
	lastReset time.Time
	mu        sync.Mutex
}

// rateLimiter manages rate limiting buckets.
type rateLimiter struct {
	buckets   map[string]*bucket
	mu        sync.RWMutex
	config    RateLimiterConfig
	cleanupCh chan struct{} // Channel to signal cleanup goroutine to stop
}

// DefaultRateLimiterConfig returns default rate limiter configuration.
func DefaultRateLimiterConfig() RateLimiterConfig {
	return RateLimiterConfig{
		Max:          100,
		Window:       time.Minute,
		KeyFunc:      defaultKeyFunc,
		ErrorMessage: "",
		StatusCode:   http.StatusTooManyRequests,
		Headers:      true,
	}
}

// defaultKeyFunc returns the client IP as the rate limit key.
func defaultKeyFunc(c *ginji.Context) string {
	// Use RemoteAddr directly - don't trust X-Forwarded-For without validation
	return c.Req.RemoteAddr
}

// keyFuncWithTrustedProxies creates a key function that validates X-Forwarded-For.
func keyFuncWithTrustedProxies(trustedProxies []string) func(*ginji.Context) string {
	return func(c *ginji.Context) string {
		// Get remote address
		remoteIP := c.Req.RemoteAddr

		// Check if remote IP is a trusted proxy
		isTrusted := false
		for _, proxy := range trustedProxies {
			if remoteIP == proxy || isIPInCIDR(remoteIP, proxy) {
				isTrusted = true
				break
			}
		}

		// Only use X-Forwarded-For if from trusted proxy
		if isTrusted {
			if ip := c.Header("X-Forwarded-For"); ip != "" {
				// Return first IP (original client)
				if idx := strings.Index(ip, ","); idx != -1 {
					return strings.TrimSpace(ip[:idx])
				}
				return ip
			}
			if ip := c.Header("X-Real-IP"); ip != "" {
				return ip
			}
		}

		return remoteIP
	}
}

// RateLimit returns a rate limiter middleware with specified max requests and window.
func RateLimit(max int, window time.Duration) ginji.Middleware {
	config := DefaultRateLimiterConfig()
	config.Max = max
	config.Window = window
	return RateLimitWithConfig(config)
}

// RateLimitWithConfig returns a rate limiter middleware with custom configuration.
func RateLimitWithConfig(config RateLimiterConfig) ginji.Middleware {
	// Set defaults
	if config.Max <= 0 {
		config.Max = 100
	}
	if config.Window <= 0 {
		config.Window = time.Minute
	}
	if config.KeyFunc == nil {
		config.KeyFunc = defaultKeyFunc
	}
	if config.StatusCode == 0 {
		config.StatusCode = http.StatusTooManyRequests
	}
	if config.ErrorMessage == "" {
		config.ErrorMessage = fmt.Sprintf("Rate limit exceeded. Maximum %d requests per %v", config.Max, config.Window)
	}
	// Setup key function with trusted proxies if configured
	if len(config.TrustedProxies) > 0 {
		// Override the default key function to use trusted proxy validation
		config.KeyFunc = keyFuncWithTrustedProxies(config.TrustedProxies)
	}

	limiter := &rateLimiter{
		buckets:   make(map[string]*bucket),
		config:    config,
		cleanupCh: make(chan struct{}),
	}

	// Start cleanup goroutine with proper lifecycle management
	go limiter.cleanup()

	return func(c *ginji.Context) error {
		// Skip if skip function returns true
		if config.SkipFunc != nil && config.SkipFunc(c) {
			return c.Next()
		}

		// Get the key for this request
		key := config.KeyFunc(c)

		// Check rate limit
		allowed, remaining, resetTime := limiter.allow(key)

		// Add rate limit headers if enabled
		if config.Headers {
			c.SetHeader("X-RateLimit-Limit", fmt.Sprintf("%d", config.Max))
			c.SetHeader("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
			c.SetHeader("X-RateLimit-Reset", fmt.Sprintf("%d", resetTime.Unix()))
		}

		if !allowed {
			c.SetHeader("Retry-After", fmt.Sprintf("%d", int(time.Until(resetTime).Seconds())))
			c.AbortWithStatusJSON(config.StatusCode, ginji.H{
				"error":   config.ErrorMessage,
				"limit":   config.Max,
				"window":  config.Window.String(),
				"retryAt": resetTime.Format(time.RFC3339),
			})
			return nil // Changed return to nil as AbortWithStatusJSON handles the response
		}

		return c.Next()
	}
}

// allow checks if a request is allowed and returns the remaining count and reset time.
func (rl *rateLimiter) allow(key string) (bool, int, time.Time) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create bucket
	b, exists := rl.buckets[key]
	if !exists {
		b = &bucket{
			tokens:    rl.config.Max,
			lastReset: now,
		}
		rl.buckets[key] = b
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	// Reset bucket if window has passed
	if now.Sub(b.lastReset) >= rl.config.Window {
		b.tokens = rl.config.Max
		b.lastReset = now
	}

	resetTime := b.lastReset.Add(rl.config.Window)

	// Check if tokens are available
	if b.tokens > 0 {
		b.tokens--
		return true, b.tokens, resetTime
	}

	return false, 0, resetTime
}

// cleanup removes old buckets periodically.
func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(rl.config.Window)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			now := time.Now()
			for key, b := range rl.buckets {
				b.mu.Lock()
				if now.Sub(b.lastReset) > rl.config.Window*2 {
					delete(rl.buckets, key)
				}
				b.mu.Unlock()
			}
			rl.mu.Unlock()
		case <-rl.cleanupCh:
			// Cleanup signal received, stop the goroutine
			return
		}
	}
}

// Stop stops the cleanup goroutine and releases resources.
func (rl *rateLimiter) Stop() {
	close(rl.cleanupCh)
}

// RateLimitPerSecond returns middleware that limits requests per second.
func RateLimitPerSecond(max int) ginji.Middleware {
	return RateLimit(max, time.Second)
}

// RateLimitPerMinute returns middleware that limits requests per minute.
func RateLimitPerMinute(max int) ginji.Middleware {
	return RateLimit(max, time.Minute)
}

// RateLimitPerHour returns middleware that limits requests per hour.
func RateLimitPerHour(max int) ginji.Middleware {
	return RateLimit(max, time.Hour)
}

// RateLimitByUser returns middleware that limits by user ID from context.
func RateLimitByUser(max int, window time.Duration, userKey string) ginji.Middleware {
	config := DefaultRateLimiterConfig()
	config.Max = max
	config.Window = window
	config.KeyFunc = func(c *ginji.Context) string {
		if userID := c.GetString(userKey); userID != "" {
			return "user:" + userID
		}
		return defaultKeyFunc(c)
	}
	return RateLimitWithConfig(config)
}

// RateLimitByAPIKey returns middleware that limits by API key header.
func RateLimitByAPIKey(max int, window time.Duration, headerName string) ginji.Middleware {
	config := DefaultRateLimiterConfig()
	config.Max = max
	config.Window = window
	config.KeyFunc = func(c *ginji.Context) string {
		if apiKey := c.Header(headerName); apiKey != "" {
			return "apikey:" + apiKey
		}
		return defaultKeyFunc(c)
	}
	return RateLimitWithConfig(config)
}
