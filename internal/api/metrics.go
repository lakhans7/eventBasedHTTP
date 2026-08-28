package api

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// Metrics is a minimal in-process counter registry exposed in Prometheus
// text exposition format at /metrics (docs/architecture.md section 24).
// A full client_golang instrumentation library was deliberately not added
// for an MVP with this few metrics — see "don't introduce unnecessary
// libraries" in the project's development rules.
type Metrics struct {
	mu            sync.Mutex
	requestsTotal map[string]int64 // key: method|route|status
	aiCallsTotal  int64
	aiCallErrors  int64
	syncFailures  int64
}

func NewMetrics() *Metrics {
	return &Metrics{requestsTotal: map[string]int64{}}
}

func (m *Metrics) ObserveRequest(method, route string, status int) {
	key := fmt.Sprintf("%s|%s|%d", method, route, status)
	m.mu.Lock()
	m.requestsTotal[key]++
	m.mu.Unlock()
}

func (m *Metrics) ObserveAICall(err error) {
	m.mu.Lock()
	m.aiCallsTotal++
	if err != nil {
		m.aiCallErrors++
	}
	m.mu.Unlock()
}

func (m *Metrics) ObserveSyncFailure() {
	m.mu.Lock()
	m.syncFailures++
	m.mu.Unlock()
}

func (m *Metrics) Middleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next()
		route := c.Route().Path
		if route == "" {
			route = c.Path()
		}
		m.ObserveRequest(c.Method(), route, c.Response().StatusCode())
		return err
	}
}

func (m *Metrics) Handler() fiber.Handler {
	return func(c *fiber.Ctx) error {
		m.mu.Lock()
		defer m.mu.Unlock()

		var b strings.Builder
		b.WriteString("# HELP http_requests_total Total HTTP requests processed.\n# TYPE http_requests_total counter\n")
		for key, count := range m.requestsTotal {
			parts := strings.SplitN(key, "|", 3)
			fmt.Fprintf(&b, "http_requests_total{method=%q,route=%q,status=%q} %d\n", parts[0], parts[1], parts[2], count)
		}
		fmt.Fprintf(&b, "# HELP ai_calls_total Total AI generation calls attempted.\n# TYPE ai_calls_total counter\nai_calls_total %d\n", m.aiCallsTotal)
		fmt.Fprintf(&b, "# HELP ai_call_errors_total Total AI generation calls that failed.\n# TYPE ai_call_errors_total counter\nai_call_errors_total %d\n", m.aiCallErrors)
		fmt.Fprintf(&b, "# HELP sync_failures_total Total failed Fiverr data imports.\n# TYPE sync_failures_total counter\nsync_failures_total %d\n", m.syncFailures)

		c.Set(fiber.HeaderContentType, "text/plain; version=0.0.4")
		return c.SendString(b.String())
	}
}
