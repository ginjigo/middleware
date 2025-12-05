package middleware

import (
	"fmt"
	"testing"
	"time"

	"github.com/ginjigo/ginji"
)

func TestRateLimitBasic(t *testing.T) {
	app := ginji.New()
	app.Use(RateLimit(3, time.Second)) // 3 requests per second

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		w := ginji.PerformRequest(app, "GET", "/test", nil)
		if w.Code != ginji.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should be rate limited
	w := ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusTooManyRequests {
		t.Errorf("Expected status 429 for rate limited request, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "Rate limit exceeded")
}

func TestRateLimitHeaders(t *testing.T) {
	app := ginji.New()
	app.Use(RateLimit(5, time.Second))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	w := ginji.PerformRequest(app, "GET", "/test", nil)

	// Check rate limit headers
	if w.Header().Get("X-RateLimit-Limit") != "5" {
		t.Errorf("Expected X-RateLimit-Limit: 5, got %s", w.Header().Get("X-RateLimit-Limit"))
	}

	if w.Header().Get("X-RateLimit-Remaining") == "" {
		t.Error("Expected X-RateLimit-Remaining header to be set")
	}

	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("Expected X-RateLimit-Reset header to be set")
	}
}

func TestRateLimitRetryAfter(t *testing.T) {
	app := ginji.New()
	app.Use(RateLimit(1, time.Second))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// First request succeeds
	w := ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Second request gets rate limited
	w = ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	// Check Retry-After header
	if w.Header().Get("Retry-After") == "" {
		t.Error("Expected Retry-After header to be set")
	}
}

func TestRateLimitReset(t *testing.T) {
	app := ginji.New()
	app.Use(RateLimit(2, 100*time.Millisecond)) // 2 requests per 100ms

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Use up the limit
	for i := 0; i < 2; i++ {
		w := ginji.PerformRequest(app, "GET", "/test", nil)
		if w.Code != ginji.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// Should be rate limited now
	w := ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	// Wait for window to reset
	time.Sleep(150 * time.Millisecond)

	// Should succeed again
	w = ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 after reset, got %d", w.Code)
	}
}

func TestRateLimitWithConfig(t *testing.T) {
	app := ginji.New()

	config := RateLimiterConfig{
		Max:          5,
		Window:       time.Second,
		ErrorMessage: "Custom error message",
		StatusCode:   ginji.StatusForbidden,
		Headers:      false, // Disable headers
	}
	app.Use(RateLimitWithConfig(config))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Use up the limit
	for i := 0; i < 5; i++ {
		ginji.PerformRequest(app, "GET", "/test", nil)
	}

	// Should be rate limited with custom config
	w := ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusForbidden {
		t.Errorf("Expected custom status code 403, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "Custom error message")

	// Headers should not be present
	if w.Header().Get("X-RateLimit-Limit") != "" {
		t.Error("Expected no rate limit headers when disabled")
	}
}

func TestRateLimitSkipFunc(t *testing.T) {
	app := ginji.New()

	config := RateLimiterConfig{
		Max:    1,
		Window: time.Second,
		SkipFunc: func(c *ginji.Context) bool {
			return c.Query("skip") == "true"
		},
	}
	app.Use(RateLimitWithConfig(config))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// First request succeeds
	w := ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Second request should be rate limited
	w = ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}

	// Request with skip=true should bypass rate limit
	w = ginji.PerformRequest(app, "GET", "/test?skip=true", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 for skipped request, got %d", w.Code)
	}
}

func TestRateLimitHelpers(t *testing.T) {
	tests := []struct {
		name       string
		middleware ginji.Middleware
		max        int
		window     time.Duration
	}{
		{"PerSecond", RateLimitPerSecond(10), 10, time.Second},
		{"PerMinute", RateLimitPerMinute(100), 100, time.Minute},
		{"PerHour", RateLimitPerHour(1000), 1000, time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := ginji.New()
			app.Use(tt.middleware)

			app.Get("/test", func(c *ginji.Context) error {
				return c.Text(ginji.StatusOK, "ok")
			})

			// Should allow requests
			w := ginji.PerformRequest(app, "GET", "/test", nil)
			if w.Code != ginji.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			// Check limit header
			if w.Header().Get("X-RateLimit-Limit") != fmt.Sprintf("%d", tt.max) {
				t.Errorf("Expected limit %d, got %s", tt.max, w.Header().Get("X-RateLimit-Limit"))
			}
		})
	}
}

func TestRateLimitByUser(t *testing.T) {
	app := ginji.New()
	app.Use(RateLimitByUser(2, time.Second, "user_id"))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Simulate user context
	req1 := ginji.NewRequest(app, "GET", "/test")

	// First 2 requests should succeed
	for i := 0; i < 2; i++ {
		w := req1.Do()
		if w.Code != ginji.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}
}

func TestRateLimitByAPIKey(t *testing.T) {
	app := ginji.New()
	app.Use(RateLimitByAPIKey(3, time.Second, "X-API-Key"))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Requests with API key
	for i := 0; i < 3; i++ {
		w := ginji.NewRequest(app, "GET", "/test").
			Header("X-API-Key", "test-key-123").
			Do()
		if w.Code != ginji.StatusOK {
			t.Errorf("Request %d: Expected status 200, got %d", i+1, w.Code)
		}
	}

	// 4th request should be rate limited
	w := ginji.NewRequest(app, "GET", "/test").
		Header("X-API-Key", "test-key-123").
		Do()
	if w.Code != ginji.StatusTooManyRequests {
		t.Errorf("Expected status 429, got %d", w.Code)
	}
}

func TestDefaultRateLimiterConfig(t *testing.T) {
	config := DefaultRateLimiterConfig()

	if config.Max != 100 {
		t.Errorf("Expected default max 100, got %d", config.Max)
	}

	if config.Window != time.Minute {
		t.Errorf("Expected default window 1 minute, got %v", config.Window)
	}

	if config.StatusCode != ginji.StatusTooManyRequests {
		t.Errorf("Expected default status 429, got %d", config.StatusCode)
	}

	if !config.Headers {
		t.Error("Expected headers to be enabled by default")
	}
}
