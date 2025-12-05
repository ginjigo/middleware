package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	"strings"

	"github.com/ginjigo/ginji"
)

// AuthConfig defines the configuration for authentication middleware.
type AuthConfig struct {
	// Validator is a function that validates credentials and returns user info.
	Validator func(credentials string) (any, bool)

	// Realm is the authentication realm for Basic Auth.
	Realm string

	// Unauthorized is called when authentication fails.
	// If nil, a default 401 response is sent.
	Unauthorized func(*ginji.Context)

	// ContextKey is the key used to store authenticated user in context.
	ContextKey string

	// SkipFunc allows skipping authentication for certain requests.
	SkipFunc func(*ginji.Context) bool
}

// BasicAuthConfig defines configuration for Basic Authentication.
type BasicAuthConfig struct {
	// Users is a map of allowed username:password pairs.
	Users map[string]string

	// Validator is a custom function to validate username/password.
	// Takes precedence over Users map.
	Validator func(username, password string) bool

	// Realm for WWW-Authenticate header.
	Realm string

	// ContextKey to store authenticated username.
	ContextKey string
}

// BearerAuthConfig defines configuration for Bearer token authentication.
type BearerAuthConfig struct {
	// Validator validates the bearer token and returns user info.
	Validator func(token string) (any, bool)

	// ContextKey to store authenticated user.
	ContextKey string

	// Realm for WWW-Authenticate header.
	Realm string
}

// APIKeyConfig defines configuration for API Key authentication.
type APIKeyConfig struct {
	// Header name to look for the API key (e.g., "X-API-Key").
	Header string

	// Query parameter name to look for the API key (optional).
	Query string

	// Validator validates the API key and returns user info.
	Validator func(key string) (any, bool)

	// ContextKey to store authenticated user.
	ContextKey string
}

// BasicAuth returns middleware for HTTP Basic Authentication.
func BasicAuth(users map[string]string) ginji.Middleware {
	return BasicAuthWithConfig(BasicAuthConfig{
		Users:      users,
		Realm:      "Authorization Required",
		ContextKey: "user",
	})
}

// BasicAuthWithConfig returns middleware with custom Basic Auth configuration.
func BasicAuthWithConfig(config BasicAuthConfig) ginji.Middleware {
	if config.Realm == "" {
		config.Realm = "Authorization Required"
	}
	if config.ContextKey == "" {
		config.ContextKey = "user"
	}

	return func(c *ginji.Context) error {
		auth := c.Header("Authorization")

		if auth == "" {
			unauthorized(c, config.Realm)
			return nil
		}

		// Parse Basic Auth header
		const prefix = "Basic "
		if !strings.HasPrefix(auth, prefix) {
			unauthorized(c, config.Realm)
			return nil
		}

		decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
		if err != nil {
			unauthorized(c, config.Realm)
			return nil
		}

		credentials := string(decoded)
		parts := strings.SplitN(credentials, ":", 2)
		if len(parts) != 2 {
			unauthorized(c, config.Realm)
			return nil
		}

		username, password := parts[0], parts[1]

		// Validate credentials
		var valid bool
		if config.Validator != nil {
			valid = config.Validator(username, password)
		} else if config.Users != nil {
			expectedPassword, exists := config.Users[username]
			valid = exists && subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
		}

		if !valid {
			unauthorized(c, config.Realm)
			return nil
		}

		// Store username in context
		c.Set(config.ContextKey, username)
		return c.Next()
	}
}

// BearerAuth returns middleware for Bearer token authentication.
func BearerAuth(validator func(token string) (any, bool)) ginji.Middleware {
	return BearerAuthWithConfig(BearerAuthConfig{
		Validator:  validator,
		ContextKey: "user",
		Realm:      "Authorization Required",
	})
}

// BearerAuthWithConfig returns middleware with custom Bearer Auth configuration.
func BearerAuthWithConfig(config BearerAuthConfig) ginji.Middleware {
	if config.ContextKey == "" {
		config.ContextKey = "user"
	}
	if config.Realm == "" {
		config.Realm = "Authorization Required"
	}

	return func(c *ginji.Context) error {
		auth := c.Header("Authorization")

		if auth == "" {
			unauthorizedBearer(c, config.Realm)
			return nil
		}

		// Parse Bearer token
		const prefix = "Bearer "
		if !strings.HasPrefix(auth, prefix) {
			unauthorizedBearer(c, config.Realm)
			return nil
		}

		token := auth[len(prefix):]
		if token == "" {
			unauthorizedBearer(c, config.Realm)
			return nil
		}

		// Validate token
		user, valid := config.Validator(token)
		if !valid {
			unauthorizedBearer(c, config.Realm)
			return nil
		}

		// Store user in context
		c.Set(config.ContextKey, user)
		return c.Next()
	}
}

// APIKey returns middleware for API Key authentication.
func APIKey(header string, validator func(key string) (any, bool)) ginji.Middleware {
	return APIKeyWithConfig(APIKeyConfig{
		Header:     header,
		Validator:  validator,
		ContextKey: "user",
	})
}

// APIKeyWithConfig returns middleware with custom API Key configuration.
func APIKeyWithConfig(config APIKeyConfig) ginji.Middleware {
	if config.ContextKey == "" {
		config.ContextKey = "user"
	}

	return func(c *ginji.Context) error {
		var apiKey string

		// Try header first
		if config.Header != "" {
			apiKey = c.Header(config.Header)
		}

		// Fall back to query parameter
		if apiKey == "" && config.Query != "" {
			apiKey = c.Query(config.Query)
		}

		if apiKey == "" {
			c.AbortWithStatusJSON(ginji.StatusUnauthorized, ginji.H{
				"error": "API key required",
			})
			return nil
		}

		// Validate API key
		user, valid := config.Validator(apiKey)
		if !valid {
			c.AbortWithStatusJSON(ginji.StatusUnauthorized, ginji.H{
				"error": "Invalid API key",
			})
			return nil
		}

		// Store user in context
		c.Set(config.ContextKey, user)
		return c.Next()
	}
}

// unauthorized sends a 401 Unauthorized response for Basic Auth.
func unauthorized(c *ginji.Context, realm string) {
	c.SetHeader("WWW-Authenticate", `Basic realm="`+realm+`"`)
	c.AbortWithStatusJSON(ginji.StatusUnauthorized, ginji.H{
		"error": "Unauthorized",
	})
}

// unauthorizedBearer sends a 401 Unauthorized response for Bearer Auth.
func unauthorizedBearer(c *ginji.Context, realm string) {
	c.SetHeader("WWW-Authenticate", `Bearer realm="`+realm+`"`)
	c.AbortWithStatusJSON(ginji.StatusUnauthorized, ginji.H{
		"error": "Unauthorized",
	})
}

// RequireRole returns middleware that checks if user has a specific role.
// Expects user to be a map[string]any with a "role" or "roles" field.
func RequireRole(role string) ginji.Middleware {
	return func(c *ginji.Context) error {
		user, exists := c.Get("user")
		if !exists {
			c.AbortWithStatusJSON(ginji.StatusForbidden, ginji.H{
				"error": "Access denied",
			})
			return nil
		}

		// Check if user has the required role
		hasRole := false
		if userMap, ok := user.(map[string]any); ok {
			// Check single role field
			if userRole, ok := userMap["role"].(string); ok && userRole == role {
				hasRole = true
			}

			// Check roles array
			if roles, ok := userMap["roles"].([]string); ok {
				for _, r := range roles {
					if r == role {
						hasRole = true
						break
					}
				}
			}
		}

		if !hasRole {
			c.AbortWithStatusJSON(ginji.StatusForbidden, ginji.H{
				"error": "Insufficient permissions",
			})
			return nil
		}

		return c.Next()
	}
}
