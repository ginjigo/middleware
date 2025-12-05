package middleware

import (
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ginjigo/ginji"
)

// TestTimeoutNoRaceCondition verifies that timeout middleware doesn't cause race conditions
// when handlers modify context state concurrently
func TestTimeoutNoRaceCondition(t *testing.T) {
	app := ginji.New()

	app.Use(Timeout(100 * time.Millisecond))

	app.Get("/race-test", func(c *ginji.Context) error {
		// Simulate handler that modifies context state
		c.Set("key1", "value1")
		c.Set("key2", "value2")

		// Read from context
		_, _ = c.Get("key1")

		// Modify params (normally set by router, but testing concurrent access)
		c.Params["test"] = "value"
		_ = c.Param("test")

		// Fast response
		time.Sleep(10 * time.Millisecond)
		return c.JSON(200, ginji.H{"status": "ok"})
	})

	// Run multiple concurrent requests to detect race conditions
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/race-test", nil)
			w := httptest.NewRecorder()
			app.ServeHTTP(w, req)
		}()
	}

	wg.Wait()
}

// TestTimeoutActualTimeout tests that timeout actually triggers
func TestTimeoutActualTimeout(t *testing.T) {
	app := ginji.New()

	app.Use(Timeout(50 * time.Millisecond))

	app.Get("/slow", func(c *ginji.Context) error {
		// Modify context state before timeout
		c.Set("before-timeout", "value")

		// Sleep longer than timeout
		time.Sleep(100 * time.Millisecond)

		// This should not execute due to timeout
		return c.JSON(200, ginji.H{"status": "completed"})
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	if w.Code != ginji.StatusGatewayTimeout {
		t.Errorf("Expected status %d, got %d", ginji.StatusGatewayTimeout, w.Code)
	}
}

// TestTimeoutContextModification tests that context modifications in timed-out handlers
// don't affect the main context
func TestTimeoutContextModification(t *testing.T) {
	app := ginji.New()

	var capturedKeys map[string]any

	app.Use(func(c *ginji.Context) error {
		err := c.Next()
		// Capture keys after timeout middleware
		capturedKeys = c.Keys
		return err
	})

	app.Use(Timeout(30 * time.Millisecond))

	app.Get("/modify", func(c *ginji.Context) error {
		// This runs in goroutine with deep copy
		c.Set("goroutine-key", "should-not-leak")
		time.Sleep(100 * time.Millisecond) // Ensure timeout
		c.Set("another-key", "also-should-not-leak")
		return nil
	})

	req := httptest.NewRequest("GET", "/modify", nil)
	w := httptest.NewRecorder()
	app.ServeHTTP(w, req)

	// Give goroutine time to finish
	time.Sleep(150 * time.Millisecond)

	// Keys set in the timed-out goroutine should not leak to main context
	if capturedKeys == nil {
		// Keys might not be initialized
		return
	}

	if _, exists := capturedKeys["goroutine-key"]; exists {
		t.Error("Goroutine key leaked to main context - deep copy not working")
	}

	if _, exists := capturedKeys["another-key"]; exists {
		t.Error("Another goroutine key leaked to main context - deep copy not working")
	}
}

// TestDeepCopyIndependence tests that DeepCopy creates independent copies
func TestDeepCopyIndependence(t *testing.T) {
	app := ginji.New()

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	original := ginji.NewContext(w, req, app)
	original.Set("key1", "original-value")
	original.Params["id"] = "123"

	// Create deep copy
	copied := original.DeepCopy()

	// Modify original
	original.Set("key1", "modified-value")
	original.Set("key2", "new-value")
	original.Params["id"] = "456"
	original.Params["name"] = "test"

	// Verify copied values are independent
	if val, ok := copied.Keys["key1"]; !ok || val != "original-value" {
		t.Error("Deep copy Keys not independent - modification affected copy")
	}

	if _, ok := copied.Keys["key2"]; ok {
		t.Error("New key in original appeared in copy")
	}

	if copied.Params["id"] != "123" {
		t.Error("Deep copy Params not independent - modification affected copy")
	}

	if _, ok := copied.Params["name"]; ok {
		t.Error("New param in original appeared in copy")
	}
}
