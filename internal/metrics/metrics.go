// Package metrics defines Prometheus instrumentation for imget.
//
// All metrics live on the default Prometheus registry; expose them on the
// HTTP server's /metrics route via promhttp.Handler().
//
// Naming follows Prometheus conventions:
//   - _total suffix for counters
//   - _seconds suffix for histograms measuring duration
//   - _bytes suffix for histograms measuring size
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// HTTPRequestsTotal counts every request reaching a handler, labelled
	// by route group and status class.
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "imget_http_requests_total",
			Help: "Total HTTP requests served, by route group and status class.",
		},
		[]string{"route", "status"},
	)

	// HTTPRequestDuration measures end-to-end request handling time.
	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "imget_http_request_duration_seconds",
			Help:    "Request handling latency, by route group.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"route"},
	)

	// RenderDuration measures the cold-render path (decode + resize + encode).
	RenderDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "imget_render_duration_seconds",
			Help:    "libvips render latency, by output format.",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5},
		},
		[]string{"format"},
	)

	// RenderTotal counts every render; status is "ok" or "err".
	RenderTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "imget_render_total",
			Help: "Total renders attempted, by format and status.",
		},
		[]string{"format", "status"},
	)

	// FetchTotal counts upstream fetches (Pixabay/Picsum), per provider.
	FetchTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "imget_fetch_total",
			Help: "Upstream image fetches, by provider and status.",
		},
		[]string{"provider", "status"},
	)

	// R2UploadTotal counts R2 uploads from the worker pool.
	R2UploadTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "imget_r2_upload_total",
			Help: "R2 uploads, by status.",
		},
		[]string{"status"},
	)

	// R2UploadBytes histograms upload sizes.
	R2UploadBytes = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "imget_r2_upload_bytes",
			Help:    "R2 upload payload size in bytes.",
			Buckets: prometheus.ExponentialBuckets(1024, 4, 8), // 1KB → ~16MB
		},
	)

	// QueueDepth tracks current backlog in async queues.
	QueueDepth = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "imget_queue_depth",
			Help: "Current depth of in-process queues.",
		},
		[]string{"queue"},
	)

	// CacheHits / CacheMisses for in-process caches (url_cache LRU,
	// render-locks LRU, etc.).
	CacheHits = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "imget_cache_hits_total",
			Help: "In-process cache hit count.",
		},
		[]string{"cache"},
	)
	CacheMisses = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "imget_cache_misses_total",
			Help: "In-process cache miss count.",
		},
		[]string{"cache"},
	)
)

// MustRegister wires every metric defined above onto the default registry.
// Idempotent — calling twice will panic, so call exactly once at startup.
func MustRegister() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		RenderDuration,
		RenderTotal,
		FetchTotal,
		R2UploadTotal,
		R2UploadBytes,
		QueueDepth,
		CacheHits,
		CacheMisses,
	)
}

// Handler returns the standard Prometheus exposition handler.
func Handler() http.Handler { return promhttp.Handler() }

// HTTPMiddleware wraps a handler and records request count + latency.
// `route` should be a low-cardinality grouping (NOT raw URL).
func HTTPMiddleware(route string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(recorder, r)
		HTTPRequestsTotal.WithLabelValues(route, statusClass(recorder.status)).Inc()
		HTTPRequestDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
	})
}

// ObserveRender records a render outcome and its duration.
func ObserveRender(format string, dur time.Duration, err error) {
	status := "ok"
	if err != nil {
		status = "err"
	}
	RenderTotal.WithLabelValues(format, status).Inc()
	RenderDuration.WithLabelValues(format).Observe(dur.Seconds())
}

func statusClass(code int) string {
	switch {
	case code >= 200 && code < 300:
		return "2xx"
	case code >= 300 && code < 400:
		return "3xx"
	case code >= 400 && code < 500:
		return "4xx"
	case code >= 500:
		return "5xx"
	}
	return strconv.Itoa(code)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(c int) {
	if !s.wrote {
		s.status = c
		s.wrote = true
	}
	s.ResponseWriter.WriteHeader(c)
}
