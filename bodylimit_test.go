package middleware

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/ginjigo/ginji"
)

func TestBodyLimit(t *testing.T) {
	app := ginji.New()
	app.Use(BodyLimit(10)) // 10 bytes limit

	app.Post("/test", func(c *ginji.Context) error {
		var data map[string]string
		if err := c.BindJSON(&data); err != nil {
			return c.JSON(ginji.StatusBadRequest, ginji.H{"error": err.Error()})
		}
		return c.JSON(ginji.StatusOK, data)
	})

	// Test within limit
	w := ginji.PerformJSONRequest(app, "POST", "/test", map[string]string{"a": "b"})
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 for small payload, got %d", w.Code)
	}

	// Test exceeding limit (via Content-Length)
	largePayload := strings.Repeat("x", 100)
	req := ginji.NewRequest(app, "POST", "/test").
		Body(bytes.NewBufferString(largePayload)).
		Header("Content-Type", "application/json")
	w = req.Do()

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413 for large payload, got %d", w.Code)
	}
}

func TestBodyLimitContentLength(t *testing.T) {
	app := ginji.New()
	app.Use(BodyLimit(50)) // 50 bytes limit

	app.Post("/test", func(c *ginji.Context) error {
		return c.JSON(ginji.StatusOK, ginji.H{"status": "ok"})
	})

	// Create request with Content-Length header exceeding limit
	largeBody := strings.Repeat("a", 100)
	req := ginji.NewRequest(app, "POST", "/test").
		Body(strings.NewReader(largeBody)).
		Header("Content-Type", "text/plain")

	w := req.Do()

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected status 413 when Content-Length exceeds limit, got %d", w.Code)
	}
}

func TestBodyLimitWithConfig(t *testing.T) {
	app := ginji.New()

	config := BodyLimitConfig{
		MaxBytes:     20,
		ErrorMessage: "Too big!",
		StatusCode:   http.StatusBadRequest,
	}
	app.Use(BodyLimitWithConfig(config))

	app.Post("/test", func(c *ginji.Context) error {
		return c.JSON(ginji.StatusOK, ginji.H{"status": "ok"})
	})

	// Test exceeding custom limit
	largeBody := strings.Repeat("x", 100)
	w := ginji.NewRequest(app, "POST", "/test").
		Body(strings.NewReader(largeBody)).
		Do()

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected custom status code 400, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "Too big!")
}

func TestBodyLimitHelpers(t *testing.T) {
	tests := []struct {
		name       string
		middleware ginji.Middleware
		expected   int64
	}{
		{"1MB", BodyLimit1MB(), 1 << 20},
		{"5MB", BodyLimit5MB(), 5 << 20},
		{"10MB", BodyLimit10MB(), 10 << 20},
		{"50MB", BodyLimit50MB(), 50 << 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := ginji.New()
			app.Use(tt.middleware)
			app.Post("/test", func(c *ginji.Context) error {
				return c.Text(ginji.StatusOK, "ok")
			})

			// Small request should pass
			w := ginji.PerformRequest(app, "POST", "/test", strings.NewReader("test"))
			if w.Code != ginji.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}
		})
	}
}

func TestBodyLimitNoBody(t *testing.T) {
	app := ginji.New()
	app.Use(BodyLimit(100))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// GET request with no body should work fine
	w := ginji.PerformRequest(app, "GET", "/test", nil)
	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 for GET request, got %d", w.Code)
	}
}

func TestDefaultBodyLimitConfig(t *testing.T) {
	config := DefaultBodyLimitConfig()

	if config.MaxBytes != 4<<20 {
		t.Errorf("Expected default max bytes to be 4MB, got %d", config.MaxBytes)
	}

	if config.StatusCode != http.StatusRequestEntityTooLarge {
		t.Errorf("Expected default status code 413, got %d", config.StatusCode)
	}
}
