package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"

	"github.com/ginjigo/ginji"
)

// CSRFConfig defines configuration for CSRF protection middleware.
type CSRFConfig struct {
	// TokenLength is the length of CSRF tokens in bytes.
	// Default: 32
	TokenLength int

	// TokenLookup specifies how to extract the token from the request.
	// Formats: "header:<name>", "form:<name>", "query:<name>"
	// Default: "header:X-CSRF-Token"
	TokenLookup string

	// CookieName is the name of the CSRF cookie.
	// Default: "_csrf"
	CookieName string

	// CookiePath is the path for the CSRF cookie.
	// Default: "/"
	CookiePath string

	// CookieDomain is the domain for the CSRF cookie.
	CookieDomain string

	// CookieSecure sets the Secure flag on the cookie.
	// Default: false
	CookieSecure bool

	// CookieHTTPOnly sets the HttpOnly flag on the cookie.
	// Default: true
	CookieHTTPOnly bool

	// CookieSameSite sets the SameSite attribute on the cookie.
	// Default: http.SameSiteStrictMode
	CookieSameSite http.SameSite

	// CookieMaxAge is the max age of the CSRF cookie in seconds.
	// Default: 86400 (24 hours)
	CookieMaxAge int

	// ContextKey is the key used to store the CSRF token in the context.
	// Default: "csrf"
	ContextKey string

	// ErrorHandler is called when CSRF validation fails.
	// If nil, a default 403 response is sent.
	ErrorHandler func(*ginji.Context)
}

// DefaultCSRFConfig returns default CSRF configuration.
func DefaultCSRFConfig() CSRFConfig {
	return CSRFConfig{
		TokenLength:    32,
		TokenLookup:    "header:X-CSRF-Token",
		CookieName:     "_csrf",
		CookiePath:     "/",
		CookieSecure:   false,
		CookieHTTPOnly: true,
		CookieSameSite: http.SameSiteStrictMode,
		CookieMaxAge:   86400, // 24 hours
		ContextKey:     "csrf",
	}
}

// CSRF returns a CSRF protection middleware with default configuration.
func CSRF() ginji.Middleware {
	return CSRFWithConfig(DefaultCSRFConfig())
}

// CSRFWithConfig returns a CSRF protection middleware with custom configuration.
func CSRFWithConfig(config CSRFConfig) ginji.Middleware {
	// Set defaults
	if config.TokenLength == 0 {
		config.TokenLength = 32
	}
	if config.TokenLookup == "" {
		config.TokenLookup = "header:X-CSRF-Token"
	}
	if config.CookieName == "" {
		config.CookieName = "_csrf"
	}
	if config.CookiePath == "" {
		config.CookiePath = "/"
	}
	if config.CookieMaxAge == 0 {
		config.CookieMaxAge = 86400
	}
	if config.ContextKey == "" {
		config.ContextKey = "csrf"
	}

	// Parse token lookup
	parts := strings.Split(config.TokenLookup, ":")
	if len(parts) != 2 {
		panic("CSRF: invalid TokenLookup format, expected 'source:name'")
	}
	lookupSource := parts[0]
	lookupName := parts[1]

	return func(c *ginji.Context) error {
		// Get or create token
		token := ""
		cookie, err := c.Cookie(config.CookieName)
		if err == nil && cookie.Value != "" {
			token = cookie.Value
		} else {
			// Generate new token
			token = generateCSRFToken(config.TokenLength)
		}

		// Set cookie
		http.SetCookie(c.Res, &http.Cookie{
			Name:     config.CookieName,
			Value:    token,
			Path:     config.CookiePath,
			Domain:   config.CookieDomain,
			MaxAge:   config.CookieMaxAge,
			Secure:   config.CookieSecure,
			HttpOnly: config.CookieHTTPOnly,
			SameSite: config.CookieSameSite,
		})

		// Store token in context for templates
		c.Set(config.ContextKey, token)

		// Skip validation for safe methods
		method := c.Req.Method
		if method == "GET" || method == "HEAD" || method == "OPTIONS" || method == "TRACE" {
			return c.Next()
		}

		// Extract token from request
		var clientToken string
		switch lookupSource {
		case "header":
			clientToken = c.Header(lookupName)
		case "form":
			clientToken = c.FormValue(lookupName)
		case "query":
			clientToken = c.Query(lookupName)
		}

		// Validate token
		if !validateCSRFToken(token, clientToken) {
			if config.ErrorHandler != nil {
				config.ErrorHandler(c)
			} else {
				c.AbortWithStatusJSON(ginji.StatusForbidden, ginji.H{
					"error": "CSRF token validation failed",
				})
			}
			return nil
		}

		return c.Next()
	}
}

// generateCSRFToken generates a random CSRF token.
func generateCSRFToken(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("failed to generate CSRF token: %v", err))
	}
	return base64.URLEncoding.EncodeToString(b)
}

// validateCSRFToken validates a CSRF token using constant-time comparison.
func validateCSRFToken(expected, actual string) bool {
	if expected == "" || actual == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

// CSRFToken is a helper to get the CSRF token from context.
func CSRFToken(c *ginji.Context) string {
	return c.GetString("csrf")
}
