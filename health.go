package middleware

import (
	"sync"
	"time"

	"github.com/ginjigo/ginji"
)

// HealthChecker is a function that checks the health of a component.
// It should return an error if the component is unhealthy.
type HealthChecker func() error

// HealthCheckConfig defines the configuration for health check middleware.
type HealthCheckConfig struct {
	// LivenessPath is the path for liveness probes.
	// Default: "/health/live"
	LivenessPath string

	// ReadinessPath is the path for readiness probes.
	// Default: "/health/ready"
	ReadinessPath string

	// Checkers are health check functions to run for readiness.
	// Liveness checks are typically simpler (just checking if the app is running).
	Checkers map[string]HealthChecker

	// Timeout is the maximum time to wait for all health checks.
	// Default: 5 seconds
	Timeout time.Duration

	// DisableLiveness disables the liveness endpoint.
	DisableLiveness bool

	// DisableReadiness disables the readiness endpoint.
	DisableReadiness bool
}

// HealthStatus represents the health status response.
type HealthStatus struct {
	Status  string            `json:"status"`
	Checks  map[string]string `json:"checks,omitempty"`
	Message string            `json:"message,omitempty"`
	Time    string            `json:"time"`
}

// DefaultHealthCheckConfig returns default health check configuration.
func DefaultHealthCheckConfig() HealthCheckConfig {
	return HealthCheckConfig{
		LivenessPath:  "/health/live",
		ReadinessPath: "/health/ready",
		Checkers:      make(map[string]HealthChecker),
		Timeout:       5 * time.Second,
	}
}

// Health returns middleware with default health check endpoints.
func Health() ginji.Middleware {
	return HealthWithConfig(DefaultHealthCheckConfig())
}

// HealthWithConfig returns middleware with custom configuration.
func HealthWithConfig(config HealthCheckConfig) ginji.Middleware {
	// Set defaults
	if config.LivenessPath == "" {
		config.LivenessPath = "/health/live"
	}
	if config.ReadinessPath == "" {
		config.ReadinessPath = "/health/ready"
	}
	if config.Timeout == 0 {
		config.Timeout = 5 * time.Second
	}
	if config.Checkers == nil {
		config.Checkers = make(map[string]HealthChecker)
	}

	return func(c *ginji.Context) error {
		path := c.Req.URL.Path

		// Liveness probe -		// Health check endpoint - checks basic app health
		if !config.DisableLiveness && path == config.LivenessPath {
			status := HealthStatus{
				Status: "UP",
				Time:   time.Now().UTC().Format(time.RFC3339),
			}
			c.JSON(ginji.StatusOK, status)
			return nil
		}

		// Readiness probe - checks if the app is ready to serve traffic
		if !config.DisableReadiness && path == config.ReadinessPath {
			handleReadiness(c, config)
			return nil
		}

		return c.Next()
	}
}

// handleReadiness handles the readiness probe request.
func handleReadiness(c *ginji.Context, config HealthCheckConfig) {
	if len(config.Checkers) == 0 {
		// No checkers configured, assume ready
		status := HealthStatus{
			Status: "UP",
			Time:   time.Now().UTC().Format(time.RFC3339),
		}
		c.JSON(ginji.StatusOK, status)
		return
	}

	// Run all health checkers with timeout
	results := make(map[string]string)
	var mu sync.Mutex
	var wg sync.WaitGroup
	done := make(chan struct{})
	allHealthy := true

	// Run checkers concurrently
	for name, checker := range config.Checkers {
		wg.Add(1)
		go func(name string, checker HealthChecker) {
			defer wg.Done()

			if err := checker(); err != nil {
				mu.Lock()
				results[name] = "DOWN: " + err.Error()
				allHealthy = false
				mu.Unlock()
			} else {
				mu.Lock()
				results[name] = "UP"
				mu.Unlock()
			}
		}(name, checker)
	}

	// Wait for all checkers or timeout
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All checkers completed
	case <-time.After(config.Timeout):
		// Timeout occurred
		allHealthy = false
		mu.Lock()
		for name := range config.Checkers {
			if _, exists := results[name]; !exists {
				results[name] = "DOWN: timeout"
			}
		}
		mu.Unlock()
	}

	// Build response - copy results map while holding lock
	mu.Lock()
	resultsCopy := make(map[string]string, len(results))
	for k, v := range results {
		resultsCopy[k] = v
	}
	mu.Unlock()

	status := HealthStatus{
		Checks: resultsCopy,
		Time:   time.Now().UTC().Format(time.RFC3339),
	}

	if allHealthy {
		status.Status = "UP"
		c.JSON(ginji.StatusOK, status)
	} else {
		status.Status = "DOWN"
		c.JSON(ginji.StatusServiceUnavailable, status)
	}
}

// AddHealthChecker adds a health checker to the configuration.
func (config *HealthCheckConfig) AddHealthChecker(name string, checker HealthChecker) {
	if config.Checkers == nil {
		config.Checkers = make(map[string]HealthChecker)
	}
	config.Checkers[name] = checker
}

// SimpleHealthCheck returns a basic health check middleware for Kubernetes-style probes.
func SimpleHealthCheck(livePath, readyPath string) ginji.Middleware {
	config := HealthCheckConfig{
		LivenessPath:  livePath,
		ReadinessPath: readyPath,
		Checkers:      make(map[string]HealthChecker),
	}
	return HealthWithConfig(config)
}
