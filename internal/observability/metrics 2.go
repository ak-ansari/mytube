// Package observability holds the Prometheus metrics and HTTP access logging for the API and worker.
// Metrics live on the default registry (which also carries Go runtime and process metrics);
// Prometheus scrapes the API at :8080/metrics and the worker at OBSERVABILITY.METRICS_ADDR.
package observability

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/ak-ansari/mytube/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mytube_http_requests_total",
		Help: "HTTP requests handled by the API, by route template and status code.",
	}, []string{"method", "route", "status"})

	httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mytube_http_request_duration_seconds",
		Help:    "HTTP request latency by route template.",
		Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
	}, []string{"method", "route"})

	httpInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "mytube_http_requests_in_flight",
		Help: "HTTP requests currently being served.",
	})

	stageRuns = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "mytube_pipeline_stage_runs_total",
		Help: "Pipeline stage handler runs, by stage and result (ok|error).",
	}, []string{"stage", "result"})

	stageDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "mytube_pipeline_stage_duration_seconds",
		Help:    "Pipeline stage handler run time.",
		Buckets: []float64{.1, .5, 1, 2.5, 5, 10, 30, 60, 120, 300, 600, 1200},
	}, []string{"stage"})
)

// HTTPMiddleware records request metrics and writes one access-log line per request.
// The route label is gin's template (/api/videos/:id), so ids don't blow up cardinality.
func HTTPMiddleware(log logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}
		start := time.Now()
		httpInFlight.Inc()
		c.Next()
		httpInFlight.Dec()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		status := c.Writer.Status()
		elapsed := time.Since(start)
		httpRequests.WithLabelValues(c.Request.Method, route, strconv.Itoa(status)).Inc()
		httpDuration.WithLabelValues(c.Request.Method, route).Observe(elapsed.Seconds())

		// always Info: the dev logger adds a stack trace to Warn/Error, and handlers already log real failures.
		// Filter by status in Loki: {service="api"} | json | status >= 500
		log.Info("http request",
			logger.String("method", c.Request.Method),
			logger.String("route", route),
			logger.String("path", c.Request.URL.Path),
			logger.Int("status", status),
			logger.Int64("duration_ms", elapsed.Milliseconds()),
			logger.String("client_ip", c.ClientIP()))
	}
}

// MetricsHandler serves the default registry.
func MetricsHandler() gin.HandlerFunc {
	return gin.WrapH(promhttp.Handler())
}

// ObserveStage times one stage handler run.
func ObserveStage(stage string, run func() error) error {
	start := time.Now()
	err := run()
	stageDuration.WithLabelValues(stage).Observe(time.Since(start).Seconds())
	result := "ok"
	if err != nil {
		result = "error"
	}
	stageRuns.WithLabelValues(stage, result).Inc()
	return err
}

// ServeMetrics exposes /metrics on addr until ctx is cancelled (for processes without an HTTP server).
func ServeMetrics(ctx context.Context, addr string, log logger.Logger) {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	go func() {
		log.Info("metrics server listening", logger.String("addr", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("metrics server stopped", logger.Error(err))
		}
	}()
}
