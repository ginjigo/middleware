package middleware

import (
	"strings"
	"testing"

	"github.com/ginjigo/ginji"
)

func TestSecureDefault(t *testing.T) {
	app := ginji.New()
	app.Use(Secure())

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "secure")
	})

	w := ginji.PerformRequest(app, "GET", "/test", nil)

	// Check default headers
	ginji.AssertHeader(t, w, "X-XSS-Protection", "1; mode=block")
	ginji.AssertHeader(t, w, "X-Content-Type-Options", "nosniff")
	ginji.AssertHeader(t, w, "X-Frame-Options", "SAMEORIGIN")
}

func TestSecureWithConfig(t *testing.T) {
	app := ginji.New()

	config := SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
		ContentSecurityPolicy: "default-src 'self'",
		ReferrerPolicy:        "no-referrer",
	}
	app.Use(SecureWithConfig(config))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "secure")
	})

	w := ginji.PerformRequest(app, "GET", "/test", nil)

	// Check custom headers
	ginji.AssertHeader(t, w, "X-Frame-Options", "DENY")
	ginji.AssertHeader(t, w, "Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
	ginji.AssertHeader(t, w, "Content-Security-Policy", "default-src 'self'")
	ginji.AssertHeader(t, w, "Referrer-Policy", "no-referrer")
}

func TestSecureStrict(t *testing.T) {
	app := ginji.New()
	app.Use(SecureStrict())

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "secure")
	})

	w := ginji.PerformRequest(app, "GET", "/test", nil)

	// Check strict headers
	ginji.AssertHeader(t, w, "X-Frame-Options", "DENY")
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Error("Expected HSTS header to be set")
	}
	ginji.AssertHeader(t, w, "Cross-Origin-Embedder-Policy", "require-corp")
	ginji.AssertHeader(t, w, "Cross-Origin-Opener-Policy", "same-origin")
	ginji.AssertHeader(t, w, "Cross-Origin-Resource-Policy", "same-origin")
}

func TestSecureCrossOriginHeaders(t *testing.T) {
	app := ginji.New()

	config := SecureConfig{
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-site",
	}
	app.Use(SecureWithConfig(config))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "test")
	})

	w := ginji.PerformRequest(app, "GET", "/test", nil)

	ginji.AssertHeader(t, w, "Cross-Origin-Embedder-Policy", "require-corp")
	ginji.AssertHeader(t, w, "Cross-Origin-Opener-Policy", "same-origin")
	ginji.AssertHeader(t, w, "Cross-Origin-Resource-Policy", "same-site")
}

func TestCSPBuilder(t *testing.T) {
	csp := NewCSP().
		DefaultSrc("'self'").
		ScriptSrc("'self'", "'unsafe-inline'").
		StyleSrc("'self'", "https://fonts.googleapis.com").
		ImgSrc("'self'", "data:", "https:").
		FontSrc("'self'", "https://fonts.gstatic.com").
		ConnectSrc("'self'").
		FrameSrc("'none'").
		ObjectSrc("'none'").
		BaseURI("'self'").
		FormAction("'self'").
		UpgradeInsecureRequests()

	policy := csp.Build()

	// Check that all directives are present
	requiredDirectives := []string{
		"default-src",
		"script-src",
		"style-src",
		"img-src",
		"font-src",
		"connect-src",
		"frame-src",
		"object-src",
		"base-uri",
		"form-action",
		"upgrade-insecure-requests",
	}

	for _, directive := range requiredDirectives {
		if !contains(policy, directive) {
			t.Errorf("Expected CSP to contain directive '%s'", directive)
		}
	}
}

func TestCSPBuilderWithMiddleware(t *testing.T) {
	csp := NewCSP().
		DefaultSrc("'self'").
		ScriptSrc("'self'").
		Build()

	app := ginji.New()

	config := SecureConfig{
		ContentSecurityPolicy: csp,
	}
	app.Use(SecureWithConfig(config))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "test")
	})

	w := ginji.PerformRequest(app, "GET", "/test", nil)

	cspHeader := w.Header().Get("Content-Security-Policy")
	if cspHeader == "" {
		t.Error("Expected Content-Security-Policy header to be set")
	}

	if !contains(cspHeader, "default-src") || !contains(cspHeader, "script-src") {
		t.Errorf("CSP header does not contain expected directives: %s", cspHeader)
	}
}

func TestDefaultSecureConfig(t *testing.T) {
	config := DefaultSecureConfig()

	if config.XSSProtection != "1; mode=block" {
		t.Errorf("Expected default XSSProtection, got %s", config.XSSProtection)
	}

	if config.ContentTypeNosniff != "nosniff" {
		t.Errorf("Expected default ContentTypeNosniff, got %s", config.ContentTypeNosniff)
	}

	if config.XFrameOptions != "SAMEORIGIN" {
		t.Errorf("Expected default XFrameOptions, got %s", config.XFrameOptions)
	}

	if config.HSTSMaxAge != 0 {
		t.Errorf("Expected default HSTSMaxAge to be 0, got %d", config.HSTSMaxAge)
	}
}

func TestSecurePermissionsPolicy(t *testing.T) {
	app := ginji.New()

	config := SecureConfig{
		PermissionsPolicy: "geolocation=(), microphone=()",
	}
	app.Use(SecureWithConfig(config))

	app.Get("/test", func(c *ginji.Context) error {
		return c.Text(ginji.StatusOK, "test")
	})

	w := ginji.PerformRequest(app, "GET", "/test", nil)

	ginji.AssertHeader(t, w, "Permissions-Policy", "geolocation=(), microphone=()")
}

// Helper function from previous tests
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			strings.Contains(s, substr)))
}
