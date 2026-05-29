// Package obs wires slog logging and Prometheus metrics.
//
// Other packages take the *slog.Logger as a constructor parameter; they
// never call slog.Default(). The Prometheus *Metrics struct is similarly
// passed into the bounded contexts that need it.
package obs

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/erfianugrah/shortr/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// NewLogger constructs the canonical slog.Logger for the process.
func NewLogger(cfg config.LogConfig) *slog.Logger {
	opts := &slog.HandlerOptions{Level: cfg.Level}
	var h slog.Handler
	switch cfg.Format {
	case "text":
		h = slog.NewTextHandler(os.Stdout, opts)
	default:
		h = slog.NewJSONHandler(os.Stdout, opts)
	}
	return slog.New(h)
}

// Metrics owns the Prometheus registry and the canonical metric instances.
type Metrics struct {
	Registry *prometheus.Registry

	HTTPRequestsTotal     *prometheus.CounterVec
	HTTPRequestDurationS  *prometheus.HistogramVec
	RedirectsTotal        *prometheus.CounterVec
	RedirectLookupErrors  prometheus.Counter
	ClicksRecordedTotal   prometheus.Counter
	ClicksDroppedTotal    prometheus.Counter
	LinksCreatedTotal     prometheus.Counter
	LinksDeletedTotal     prometheus.Counter
}

// NewMetrics builds a fresh registry with go + process collectors and
// shortr's custom counters/histograms.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	m := &Metrics{
		Registry: reg,
		HTTPRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "shortr_http_requests_total",
			Help: "HTTP requests by route + method + status.",
		}, []string{"route", "method", "status"}),
		HTTPRequestDurationS: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "shortr_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"route", "method"}),
		RedirectsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "shortr_redirects_total",
			Help: "Slug-redirect outcomes (hit | not_found | expired | exhausted).",
		}, []string{"outcome"}),
		RedirectLookupErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shortr_redirect_lookup_errors_total",
			Help: "Storage errors during redirect lookup.",
		}),
		ClicksRecordedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shortr_clicks_recorded_total",
			Help: "Click events successfully written.",
		}),
		ClicksDroppedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shortr_clicks_dropped_total",
			Help: "Click events dropped due to full buffer.",
		}),
		LinksCreatedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shortr_links_created_total",
			Help: "Links successfully created.",
		}),
		LinksDeletedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "shortr_links_deleted_total",
			Help: "Links deleted.",
		}),
	}
	reg.MustRegister(
		m.HTTPRequestsTotal,
		m.HTTPRequestDurationS,
		m.RedirectsTotal,
		m.RedirectLookupErrors,
		m.ClicksRecordedTotal,
		m.ClicksDroppedTotal,
		m.LinksCreatedTotal,
		m.LinksDeletedTotal,
	)
	return m
}

// Handler returns an http.Handler exposing /metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{Registry: m.Registry})
}
