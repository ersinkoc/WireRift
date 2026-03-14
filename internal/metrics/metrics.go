// Package metrics provides Prometheus-style metrics collection
package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// Metrics holds server metrics
type Metrics struct {
	// Counters
	connectionsTotal   int64
	connectionsActive  int64
	tunnelsTotal       int64
	tunnelsActive      int64
	requestsTotal       int64
	bytesIn            int64
	bytesOut           int64
	errorsTotal        int64

	// Gauges (current values)
	lastRequestTime     int64
	avgLatencyNs        int64

	// Start time
	startTime time.Time
}

// Collector collects metrics
type Collector struct {
	metrics Metrics
}

// NewCollector creates a new metrics collector
func NewCollector() *Collector {
	return &Collector{
		metrics: Metrics{
			startTime: time.Now(),
		},
	}
}

// IncrementConnection increments the connection counter
func (c *Collector) IncrementConnection() {
	atomic.AddInt64(&c.metrics.connectionsTotal, 1)
	atomic.AddInt64(&c.metrics.connectionsActive, 1)
}

// DecrementConnection decrements the active connection counter
func (c *Collector) DecrementConnection() {
	atomic.AddInt64(&c.metrics.connectionsActive, -1)
}

// IncrementTunnel increments the tunnel counter
func (c *Collector) IncrementTunnel() {
	atomic.AddInt64(&c.metrics.tunnelsTotal, 1)
	atomic.AddInt64(&c.metrics.tunnelsActive, 1)
}

// DecrementTunnel decrements the active tunnel counter
func (c *Collector) DecrementTunnel() {
	atomic.AddInt64(&c.metrics.tunnelsActive, -1)
}

// IncrementRequest increments the request counter
func (c *Collector) IncrementRequest() {
	atomic.AddInt64(&c.metrics.requestsTotal, 1)
	atomic.StoreInt64(&c.metrics.lastRequestTime, time.Now().UnixNano())
}

// AddBytesIn adds to the bytes received counter
func (c *Collector) AddBytesIn(n int64) {
	atomic.AddInt64(&c.metrics.bytesIn, n)
}

// AddBytesOut adds to the bytes sent counter
func (c *Collector) AddBytesOut(n int64) {
	atomic.AddInt64(&c.metrics.bytesOut, n)
}

// IncrementError increments the error counter
func (c *Collector) IncrementError() {
	atomic.AddInt64(&c.metrics.errorsTotal, 1)
}

// RecordLatency records a latency measurement
func (c *Collector) RecordLatency(d time.Duration) {
	// Simple exponential moving average
	latencyNs := d.Nanoseconds()
	for {
		old := atomic.LoadInt64(&c.metrics.avgLatencyNs)
		var newAvg int64
		if old == 0 {
			newAvg = latencyNs
		} else {
			newAvg = (old*7 + latencyNs*3) / 10
		}
		if atomic.CompareAndSwapInt64(&c.metrics.avgLatencyNs, old, newAvg) {
			break
		}
	}
}

// Snapshot returns a snapshot of all metrics
func (c *Collector) Snapshot() Metrics {
	return Metrics{
		connectionsTotal:   atomic.LoadInt64(&c.metrics.connectionsTotal),
		connectionsActive:  atomic.LoadInt64(&c.metrics.connectionsActive),
		tunnelsTotal:       atomic.LoadInt64(&c.metrics.tunnelsTotal),
		tunnelsActive:      atomic.LoadInt64(&c.metrics.tunnelsActive),
		requestsTotal:      atomic.LoadInt64(&c.metrics.requestsTotal),
		bytesIn:           atomic.LoadInt64(&c.metrics.bytesIn),
		bytesOut:          atomic.LoadInt64(&c.metrics.bytesOut),
		errorsTotal:       atomic.LoadInt64(&c.metrics.errorsTotal),
		lastRequestTime:    atomic.LoadInt64(&c.metrics.lastRequestTime),
		avgLatencyNs:      atomic.LoadInt64(&c.metrics.avgLatencyNs),
		startTime:          c.metrics.startTime,
	}
}

// Uptime returns how long the server has been running
func (c *Collector) Uptime() time.Duration {
	return time.Since(c.metrics.startTime)
}

// PrometheusFormat returns metrics in Prometheus text format
func (c *Collector) PrometheusFormat() string {
	m := c.Snapshot()
	uptime := c.Uptime().Seconds()

	return fmt.Sprintf(`# HELP WireRift Metrics
# TYPE counter
wirerift_connections_total %d
# TYPE gauge
wirerift_connections_active %d
# TYPE counter
wirerift_tunnels_total %d
# TYPE gauge
wirerift_tunnels_active %d
# TYPE counter
wirerift_requests_total %d
# TYPE counter
wirerift_bytes_in %d
# TYPE counter
wirerift_bytes_out %d
# TYPE counter
wirerift_errors_total %d
# TYPE gauge
wirerift_uptime_seconds %d

`,
		m.connectionsTotal,
		m.connectionsActive,
		m.tunnelsTotal,
		m.tunnelsActive,
		m.requestsTotal,
		m.bytesIn,
		m.bytesOut,
		m.errorsTotal,
		int64(uptime),
	)
}

// Handler returns an HTTP handler for serves Prometheus metrics
func (c *Collector) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.0")
		w.Write([]byte(c.PrometheusFormat()))
	}
}
