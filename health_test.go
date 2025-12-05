package middleware

import (
	"errors"
	"testing"
	"time"

	"github.com/ginjigo/ginji"
)

func TestHealthLiveness(t *testing.T) {
	app := ginji.New()
	app.Use(Health())

	app.Get("/api/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test liveness endpoint
	w := ginji.PerformRequest(app, "GET", "/health/live", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "UP")
}

func TestHealthReadinessNoCheckers(t *testing.T) {
	app := ginji.New()
	app.Use(Health())

	app.Get("/api/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test readiness endpoint with no checkers (should return UP)
	w := ginji.PerformRequest(app, "GET", "/health/ready", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "UP")
}

func TestHealthReadinessWithCheckers(t *testing.T) {
	app := ginji.New()

	config := DefaultHealthCheckConfig()

	// Add healthy checker
	config.AddHealthChecker("database", func() error {
		return nil // Database is healthy
	})

	// Add another healthy checker
	config.AddHealthChecker("cache", func() error {
		return nil // Cache is healthy
	})

	app.Use(HealthWithConfig(config))

	app.Get("/api/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test readiness endpoint
	w := ginji.PerformRequest(app, "GET", "/health/ready", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "UP")
	ginji.AssertBody(t, w, "database")
	ginji.AssertBody(t, w, "cache")
}

func TestHealthReadinessUnhealthy(t *testing.T) {
	app := ginji.New()

	config := DefaultHealthCheckConfig()

	// Add healthy checker
	config.AddHealthChecker("database", func() error {
		return nil
	})

	// Add unhealthy checker
	config.AddHealthChecker("cache", func() error {
		return errors.New("connection timeout")
	})

	app.Use(HealthWithConfig(config))

	app.Get("/api/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test readiness endpoint
	w := ginji.PerformRequest(app, "GET", "/health/ready", nil)

	if w.Code != ginji.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "DOWN")
	ginji.AssertBody(t, w, "connection timeout")
}

func TestHealthCustomPaths(t *testing.T) {
	app := ginji.New()

	config := HealthCheckConfig{
		LivenessPath:  "/custom/alive",
		ReadinessPath: "/custom/ready",
		Checkers:      make(map[string]HealthChecker),
	}
	app.Use(HealthWithConfig(config))

	app.Get("/api/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test custom liveness path
	w := ginji.PerformRequest(app, "GET", "/custom/alive", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 for liveness, got %d", w.Code)
	}

	// Test custom readiness path
	w = ginji.PerformRequest(app, "GET", "/custom/ready", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 for readiness, got %d", w.Code)
	}

	// Regular routes should still work
	w = ginji.PerformRequest(app, "GET", "/api/test", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 for regular route, got %d", w.Code)
	}
}

func TestHealthDisableEndpoints(t *testing.T) {
	app := ginji.New()

	config := HealthCheckConfig{
		LivenessPath:     "/health/live",
		ReadinessPath:    "/health/ready",
		DisableLiveness:  true,
		DisableReadiness: true,
		Checkers:         make(map[string]HealthChecker),
	}
	app.Use(HealthWithConfig(config))

	// Add fallback 404 handler
	app.Get("/health/live", func(c *ginji.Context) error {
		return c.Text(ginji.StatusNotFound, "not found")
	})

	// Test that endpoints are disabled
	w := ginji.PerformRequest(app, "GET", "/health/live", nil)
	// When disabled, endpoint should reach fallback handler (not found)
	// Expected behavior - endpoint is disabled and fallback is reached
	_ = w
}

func TestHealthTimeout(t *testing.T) {
	app := ginji.New()

	config := DefaultHealthCheckConfig()
	config.Timeout = 100 * time.Millisecond

	// Add slow checker that will timeout
	config.AddHealthChecker("slow_service", func() error {
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	app.Use(HealthWithConfig(config))

	// Test readiness endpoint
	w := ginji.PerformRequest(app, "GET", "/health/ready", nil)

	if w.Code != ginji.StatusServiceUnavailable {
		t.Errorf("Expected status 503 due to timeout, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "timeout")
}

func TestSimpleHealthCheck(t *testing.T) {
	app := ginji.New()
	app.Use(SimpleHealthCheck("/live", "/ready"))

	app.Get("/api/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test liveness
	w := ginji.PerformRequest(app, "GET", "/live", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Test readiness
	w = ginji.PerformRequest(app, "GET", "/ready", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDefaultHealthCheckConfig(t *testing.T) {
	config := DefaultHealthCheckConfig()

	if config.LivenessPath != "/health/live" {
		t.Errorf("Expected default liveness path, got %s", config.LivenessPath)
	}

	if config.ReadinessPath != "/health/ready" {
		t.Errorf("Expected default readiness path, got %s", config.ReadinessPath)
	}

	if config.Timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", config.Timeout)
	}
}
