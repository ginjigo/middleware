package middleware

import (
	"fmt"
	"strings"

	"github.com/ginjigo/ginji"
)

// SecureConfig defines the configuration for security headers middleware.
type SecureConfig struct {
	// XSSProtection provides XSS protection header.
	// Default: "1; mode=block"
	XSSProtection string

	// ContentTypeNosniff provides X-Content-Type-Options header.
	// Default: "nosniff"
	ContentTypeNosniff string

	// XFrameOptions provides X-Frame-Options header.
	// Possible values: "DENY", "SAMEORIGIN", "ALLOW-FROM uri"
	// Default: "SAMEORIGIN"
	XFrameOptions string

	// HSTSMaxAge sets the Strict-Transport-Security header max-age value in seconds.
	// Default: 0 (disabled)
	HSTSMaxAge int

	// HSTSIncludeSubdomains adds includeSubDomains to the HSTS header.
	// Default: false
	HSTSIncludeSubdomains bool

	// HSTSPreload adds preload to the HSTS header.
	// Default: false
	HSTSPreload bool

	// ContentSecurityPolicy sets the Content-Security-Policy header.
	// Default: "" (not set)
	ContentSecurityPolicy string

	// ReferrerPolicy sets the Referrer-Policy header.
	// Default: "" (not set)
	ReferrerPolicy string

	// PermissionsPolicy sets the Permissions-Policy header.
	// Default: "" (not set)
	PermissionsPolicy string

	// CrossOriginEmbedderPolicy sets the Cross-Origin-Embedder-Policy header.
	// Possible values: "unsafe-none", "require-corp", "credentialless"
	// Default: "" (not set)
	CrossOriginEmbedderPolicy string

	// CrossOriginOpenerPolicy sets the Cross-Origin-Opener-Policy header.
	// Possible values: "unsafe-none", "same-origin-allow-popups", "same-origin"
	// Default: "" (not set)
	CrossOriginOpenerPolicy string

	// CrossOriginResourcePolicy sets the Cross-Origin-Resource-Policy header.
	// Possible values: "same-site", "same-origin", "cross-origin"
	// Default: "" (not set)
	CrossOriginResourcePolicy string
}

// DefaultSecureConfig returns a default secure configuration.
func DefaultSecureConfig() SecureConfig {
	return SecureConfig{
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "SAMEORIGIN",
		HSTSMaxAge:         0,
	}
}

// Secure returns a middleware that sets security headers with default configuration.
func Secure() ginji.Middleware {
	return SecureWithConfig(DefaultSecureConfig())
}

// SecureWithConfig returns a middleware that sets security headers with custom configuration.
func SecureWithConfig(config SecureConfig) ginji.Middleware {
	return func(c *ginji.Context) error {
		// X-XSS-Protection
		if config.XSSProtection != "" {
			c.SetHeader("X-XSS-Protection", config.XSSProtection)
		}

		// X-Content-Type-Options
		if config.ContentTypeNosniff != "" {
			c.SetHeader("X-Content-Type-Options", config.ContentTypeNosniff)
		}

		// X-Frame-Options
		if config.XFrameOptions != "" {
			c.SetHeader("X-Frame-Options", config.XFrameOptions)
		}

		// Strict-Transport-Security
		if config.HSTSMaxAge > 0 {
			hsts := fmt.Sprintf("max-age=%d", config.HSTSMaxAge)
			if config.HSTSIncludeSubdomains {
				hsts += "; includeSubDomains"
			}
			if config.HSTSPreload {
				hsts += "; preload"
			}
			c.SetHeader("Strict-Transport-Security", hsts)
		}

		// Content-Security-Policy
		if config.ContentSecurityPolicy != "" {
			c.SetHeader("Content-Security-Policy", config.ContentSecurityPolicy)
		}

		// Referrer-Policy
		if config.ReferrerPolicy != "" {
			c.SetHeader("Referrer-Policy", config.ReferrerPolicy)
		}

		// Permissions-Policy
		if config.PermissionsPolicy != "" {
			c.SetHeader("Permissions-Policy", config.PermissionsPolicy)
		}

		// Cross-Origin-Embedder-Policy
		if config.CrossOriginEmbedderPolicy != "" {
			c.SetHeader("Cross-Origin-Embedder-Policy", config.CrossOriginEmbedderPolicy)
		}

		// Cross-Origin-Opener-Policy
		if config.CrossOriginOpenerPolicy != "" {
			c.SetHeader("Cross-Origin-Opener-Policy", config.CrossOriginOpenerPolicy)
		}

		// Cross-Origin-Resource-Policy
		if config.CrossOriginResourcePolicy != "" {
			c.SetHeader("Cross-Origin-Resource-Policy", config.CrossOriginResourcePolicy)
		}

		return c.Next()
	}
}

// SecureStrict returns middleware with strict security headers for production.
func SecureStrict() ginji.Middleware {
	config := SecureConfig{
		XSSProtection:             "1; mode=block",
		ContentTypeNosniff:        "nosniff",
		XFrameOptions:             "DENY",
		HSTSMaxAge:                31536000, // 1 year
		HSTSIncludeSubdomains:     true,
		HSTSPreload:               true,
		ContentSecurityPolicy:     "default-src 'self'",
		ReferrerPolicy:            "strict-origin-when-cross-origin",
		CrossOriginEmbedderPolicy: "require-corp",
		CrossOriginOpenerPolicy:   "same-origin",
		CrossOriginResourcePolicy: "same-origin",
	}
	return SecureWithConfig(config)
}

// CSP is a helper to build Content-Security-Policy headers.
type CSP struct {
	directives map[string][]string
}

// NewCSP creates a new CSP builder.
func NewCSP() *CSP {
	return &CSP{
		directives: make(map[string][]string),
	}
}

// DefaultSrc sets the default-src directive.
func (csp *CSP) DefaultSrc(sources ...string) *CSP {
	csp.directives["default-src"] = sources
	return csp
}

// ScriptSrc sets the script-src directive.
func (csp *CSP) ScriptSrc(sources ...string) *CSP {
	csp.directives["script-src"] = sources
	return csp
}

// StyleSrc sets the style-src directive.
func (csp *CSP) StyleSrc(sources ...string) *CSP {
	csp.directives["style-src"] = sources
	return csp
}

// ImgSrc sets the img-src directive.
func (csp *CSP) ImgSrc(sources ...string) *CSP {
	csp.directives["img-src"] = sources
	return csp
}

// FontSrc sets the font-src directive.
func (csp *CSP) FontSrc(sources ...string) *CSP {
	csp.directives["font-src"] = sources
	return csp
}

// ConnectSrc sets the connect-src directive.
func (csp *CSP) ConnectSrc(sources ...string) *CSP {
	csp.directives["connect-src"] = sources
	return csp
}

// FrameSrc sets the frame-src directive.
func (csp *CSP) FrameSrc(sources ...string) *CSP {
	csp.directives["frame-src"] = sources
	return csp
}

// ObjectSrc sets the object-src directive.
func (csp *CSP) ObjectSrc(sources ...string) *CSP {
	csp.directives["object-src"] = sources
	return csp
}

// BaseURI sets the base-uri directive.
func (csp *CSP) BaseURI(sources ...string) *CSP {
	csp.directives["base-uri"] = sources
	return csp
}

// FormAction sets the form-action directive.
func (csp *CSP) FormAction(sources ...string) *CSP {
	csp.directives["form-action"] = sources
	return csp
}

// UpgradeInsecureRequests adds the upgrade-insecure-requests directive.
func (csp *CSP) UpgradeInsecureRequests() *CSP {
	csp.directives["upgrade-insecure-requests"] = []string{}
	return csp
}

// Build constructs the CSP header value.
func (csp *CSP) Build() string {
	var parts []string
	for directive, sources := range csp.directives {
		if len(sources) == 0 {
			parts = append(parts, directive)
		} else {
			parts = append(parts, fmt.Sprintf("%s %s", directive, strings.Join(sources, " ")))
		}
	}
	return strings.Join(parts, "; ")
}
