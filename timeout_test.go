package middleware

import (
	"testing"
	"time"

	"github.com/ginjigo/ginji"
)

func TestTimeoutSuccess(t *testing.T) {
	app := ginji.New()
	app.Use(Timeout(1 * time.Second))

	app.Get("/fast", func(c *ginji.Context) error {
		// Fast handler that completes quickly
		return c.Text(ginji.StatusOK, "success")
	})

	w := ginji.PerformRequest(app, "GET", "/fast", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "success")
}

func TestTimeoutExceeded(t *testing.T) {
	app := ginji.New()
	app.Use(Timeout(100 * time.Millisecond))

	app.Get("/slow", func(c *ginji.Context) error {
		// Slow handler that exceeds timeout
		time.Sleep(200 * time.Millisecond)
		return c.Text(ginji.StatusOK, "should not reach")
	})

	w := ginji.PerformRequest(app, "GET", "/slow", nil)

	if w.Code != ginji.StatusGatewayTimeout {
		t.Errorf("Expected status 504, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "timeout")
}

func TestTimeoutWithConfig(t *testing.T) {
	app := ginji.New()

	config := TimeoutConfig{
		Timeout:      50 * time.Millisecond,
		ErrorMessage: "Custom timeout message",
		StatusCode:   ginji.StatusRequestTimeout,
	}
	app.Use(TimeoutWithConfig(config))

	app.Get("/slow", func(c *ginji.Context) error {
		time.Sleep(100 * time.Millisecond)
		return c.Text(ginji.StatusOK, "ok")
	})

	w := ginji.PerformRequest(app, "GET", "/slow", nil)

	if w.Code != ginji.StatusRequestTimeout {
		t.Errorf("Expected custom status 408, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "Custom timeout message")
}

func TestTimeoutSkipFunc(t *testing.T) {
	app := ginji.New()

	config := TimeoutConfig{
		Timeout: 50 * time.Millisecond,
		SkipFunc: func(c *ginji.Context) bool {
			return c.Query("skip") == "true"
		},
	}
	app.Use(TimeoutWithConfig(config))

	app.Get("/slow", func(c *ginji.Context) error {
		time.Sleep(100 * time.Millisecond)
		return c.Text(ginji.StatusOK, "completed")
	})

	// Request with skip should not timeout
	w := ginji.PerformRequest(app, "GET", "/slow?skip=true", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 for skipped request, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "completed")
}

func TestTimeoutContext(t *testing.T) {
	app := ginji.New()
	app.Use(Timeout(100 * time.Millisecond))

	contextChecked := false

	app.Get("/check-context", func(c *ginji.Context) error {
		// Check if context has deadline
		_, hasDeadline := c.Req.Context().Deadline()
		contextChecked = hasDeadline
		return c.Text(ginji.StatusOK, "ok")
	})

	w := ginji.PerformRequest(app, "GET", "/check-context", nil)

	if !contextChecked {
		t.Error("Expected context to have deadline")
	}

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTimeoutHelpers(t *testing.T) {
	tests := []struct {
		name       string
		middleware ginji.Middleware
		expected   time.Duration
	}{
		{"TimeoutSeconds", TimeoutSeconds(5), 5 * time.Second},
		{"TimeoutMinutes", TimeoutMinutes(2), 2 * time.Minute},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := ginji.New()
			app.Use(tt.middleware)

			app.Get("/test", func(c *ginji.Context) error {
				deadline, ok := c.Req.Context().Deadline()
				if !ok {
					t.Error("Expected context to have deadline")
					return nil
				}

				timeout := time.Until(deadline)
				if timeout < tt.expected-time.Second || timeout > tt.expected+time.Second {
					t.Errorf("Expected timeout around %v, got %v", tt.expected, timeout)
				}

				return c.Text(ginji.StatusOK, "ok")
			})

			w := ginji.PerformRequest(app, "GET", "/test", nil)
			if w.Code != ginji.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestDefaultTimeoutConfig(t *testing.T) {
	config := DefaultTimeoutConfig()

	if config.Timeout != 30*time.Second {
		t.Errorf("Expected default timeout 30s, got %v", config.Timeout)
	}

	if config.StatusCode != ginji.StatusGatewayTimeout {
		t.Errorf("Expected default status 504, got %d", config.StatusCode)
	}

	if config.ErrorMessage != "Request timeout" {
		t.Errorf("Expected default error message, got %s", config.ErrorMessage)
	}
}

func TestTimeoutNoTimeout(t *testing.T) {
	app := ginji.New()
	app.Use(Timeout(1 * time.Second))

	app.Get("/instant", func(c *ginji.Context) error {
		// Handler completes immediately
		return c.JSON(ginji.StatusOK, ginji.H{"status": "ok"})
	})

	w := ginji.PerformRequest(app, "GET", "/instant", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestTimeoutMultipleRequests(t *testing.T) {
	app := ginji.New()
	app.Use(Timeout(100 * time.Millisecond))

	app.Get("/test", func(c *ginji.Context) error {
		// Some requests fast, some slow
		if c.Query("delay") == "yes" {
			time.Sleep(150 * time.Millisecond)
		}
		return c.Text(ginji.StatusOK, "done")
	})

	// Fast request
	w1 := ginji.PerformRequest(app, "GET", "/test", nil)
	if w1.Code != ginji.StatusOK {
		t.Errorf("Fast request: Expected status 200, got %d", w1.Code)
	}

	// Slow request
	w2 := ginji.PerformRequest(app, "GET", "/test?delay=yes", nil)
	if w2.Code != ginji.StatusGatewayTimeout {
		t.Errorf("Slow request: Expected status 504, got %d", w2.Code)
	}
}
