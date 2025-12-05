package middleware

import (
	"encoding/base64"
	"testing"

	"github.com/ginjigo/ginji"
)

func TestBasicAuth(t *testing.T) {
	app := ginji.New()

	users := map[string]string{
		"admin": "secret123",
		"user":  "pass456",
	}
	app.Use(BasicAuth(users))

	app.Get("/protected", func(c *ginji.Context) error {
		username := c.GetString("user")
		return c.JSON(ginji.StatusOK, ginji.H{"user": username})
	})

	// Test valid credentials
	auth := base64.StdEncoding.EncodeToString([]byte("admin:secret123"))
	w := ginji.NewRequest(app, "GET", "/protected").
		Header("Authorization", "Basic "+auth).
		Do()

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "admin")
}

func TestBasicAuthInvalid(t *testing.T) {
	app := ginji.New()

	users := map[string]string{
		"admin": "secret123",
	}
	app.Use(BasicAuth(users))

	app.Get("/protected", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test invalid password
	auth := base64.StdEncoding.EncodeToString([]byte("admin:wrongpass"))
	w := ginji.NewRequest(app, "GET", "/protected").
		Header("Authorization", "Basic "+auth).
		Do()

	if w.Code != ginji.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	ginji.AssertHeader(t, w, "WWW-Authenticate", `Basic realm="Authorization Required"`)
}

func TestBasicAuthMissing(t *testing.T) {
	app := ginji.New()

	users := map[string]string{
		"admin": "secret123",
	}
	app.Use(BasicAuth(users))

	app.Get("/protected", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test missing authorization header
	w := ginji.PerformRequest(app, "GET", "/protected", nil)

	if w.Code != ginji.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestBasicAuthWithCustomValidator(t *testing.T) {
	app := ginji.New()

	config := BasicAuthConfig{
		Validator: func(username, password string) bool {
			return username == "custom" && password == "validator"
		},
		ContextKey: "authenticated_user",
	}
	app.Use(BasicAuthWithConfig(config))

	app.Get("/protected", func(c *ginji.Context) error {
		user := c.GetString("authenticated_user")
		return c.Text(ginji.StatusOK, user)
	})

	// Test custom validator
	auth := base64.StdEncoding.EncodeToString([]byte("custom:validator"))
	w := ginji.NewRequest(app, "GET", "/protected").
		Header("Authorization", "Basic "+auth).
		Do()

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "custom")
}

func TestBearerAuth(t *testing.T) {
	app := ginji.New()

	validator := func(token string) (any, bool) {
		if token == "valid-token-123" {
			return map[string]any{"id": "user1", "name": "John"}, true
		}
		return nil, false
	}
	app.Use(BearerAuth(validator))

	app.Get("/api/data", func(c *ginji.Context) error {
		user, _ := c.Get("user")
		return c.JSON(ginji.StatusOK, user)
	})

	// Test valid token
	w := ginji.NewRequest(app, "GET", "/api/data").
		Header("Authorization", "Bearer valid-token-123").
		Do()

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "John")
}

func TestBearerAuthInvalid(t *testing.T) {
	app := ginji.New()

	validator := func(token string) (any, bool) {
		return nil, false // Always invalid
	}
	app.Use(BearerAuth(validator))

	app.Get("/api/data", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test invalid token
	w := ginji.NewRequest(app, "GET", "/api/data").
		Header("Authorization", "Bearer invalid-token").
		Do()

	if w.Code != ginji.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	ginji.AssertHeader(t, w, "WWW-Authenticate", `Bearer realm="Authorization Required"`)
}

func TestBearerAuthMissing(t *testing.T) {
	app := ginji.New()

	validator := func(token string) (any, bool) {
		return map[string]any{"id": "user1"}, true
	}
	app.Use(BearerAuth(validator))

	app.Get("/api/data", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test missing token
	w := ginji.PerformRequest(app, "GET", "/api/data", nil)

	if w.Code != ginji.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAPIKey(t *testing.T) {
	app := ginji.New()

	validator := func(key string) (any, bool) {
		validKeys := map[string]any{
			"key-123": map[string]any{"client": "ClientA"},
			"key-456": map[string]any{"client": "ClientB"},
		}
		user, ok := validKeys[key]
		return user, ok
	}
	app.Use(APIKey("X-API-Key", validator))

	app.Get("/api/resource", func(c *ginji.Context) error {
		user, _ := c.Get("user")
		return c.JSON(ginji.StatusOK, user)
	})

	// Test valid API key
	w := ginji.NewRequest(app, "GET", "/api/resource").
		Header("X-API-Key", "key-123").
		Do()

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "ClientA")
}

func TestAPIKeyInvalid(t *testing.T) {
	app := ginji.New()

	validator := func(key string) (any, bool) {
		return nil, false
	}
	app.Use(APIKey("X-API-Key", validator))

	app.Get("/api/resource", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test invalid API key
	w := ginji.NewRequest(app, "GET", "/api/resource").
		Header("X-API-Key", "invalid-key").
		Do()

	if w.Code != ginji.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "Invalid API key")
}

func TestAPIKeyQueryParam(t *testing.T) {
	app := ginji.New()

	validator := func(key string) (any, bool) {
		return key == "query-key-789", true
	}

	config := APIKeyConfig{
		Header:     "X-API-Key",
		Query:      "api_key",
		Validator:  validator,
		ContextKey: "user",
	}
	app.Use(APIKeyWithConfig(config))

	app.Get("/api/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "ok")
	})

	// Test API key in query parameter
	w := ginji.PerformRequest(app, "GET", "/api/test?api_key=query-key-789", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 with query param API key, got %d", w.Code)
	}
}

func TestRequireRole(t *testing.T) {
	app := ginji.New()

	// Mock auth middleware that sets user
	app.Use(func(c *ginji.Context) error {
		// Simulate authenticated user with role
		c.Set("user", map[string]any{
			"id":   "user1",
			"role": "admin",
		})
		return c.Next()
	})

	app.Use(RequireRole("admin"))

	app.Get("/admin/panel", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "admin panel")
	})

	// Test user with admin role
	w := ginji.PerformRequest(app, "GET", "/admin/panel", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200 for user with admin role, got %d", w.Code)
	}
}

func TestRequireRoleInsufficientPermissions(t *testing.T) {
	app := ginji.New()

	// Mock auth middleware with different role
	app.Use(func(c *ginji.Context) error {
		c.Set("user", map[string]any{
			"id":   "user1",
			"role": "user", // Not admin
		})
		return c.Next()
	})

	app.Use(RequireRole("admin"))

	app.Get("/admin/panel", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "admin panel")
	})

	// Test user without admin role
	w := ginji.PerformRequest(app, "GET", "/admin/panel", nil)

	if w.Code != ginji.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}

	ginji.AssertBody(t, w, "Insufficient permissions")
}

func TestRequireRoleWithRolesArray(t *testing.T) {
	app := ginji.New()

	// Mock auth with roles array
	app.Use(func(c *ginji.Context) error {
		c.Set("user", map[string]any{
			"id":    "user1",
			"roles": []string{"user", "moderator"},
		})
		return c.Next()
	})

	app.Use(RequireRole("moderator"))

	app.Get("/moderate", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "moderation panel")
	})

	// Test user with moderator in roles array
	w := ginji.PerformRequest(app, "GET", "/moderate", nil)

	if w.Code != ginji.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
