package observability

import (
	"net/http"
	"strconv"
	"time"
)

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += n
	return n, err
}

// MetricsMiddleware records HTTP request metrics for every handler.
// Records: request count, duration, error count, active connections.
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ActiveHTTPConnections.Inc()
		defer ActiveHTTPConnections.Dec()

		rw := newResponseWriter(w)
		next.ServeHTTP(rw, r)

		durMs := float64(time.Since(start).Milliseconds())
		statusClass := statusClass(rw.statusCode)
		statusCode := strconv.Itoa(rw.statusCode)

		// Normalize path to avoid high-cardinality label explosion.
		path := normalizePath(r.URL.Path)

		HTTPRequestsTotal.WithLabelValues(r.Method, path, statusClass).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, path).Observe(durMs)

		if rw.statusCode >= 400 {
			HTTPErrorsTotal.WithLabelValues(r.Method, path, statusCode).Inc()
		}
	})
}

// TraceMiddleware injects a trace ID into every request context and response header.
// The trace ID propagates through logs for end-to-end correlation.
func TraceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceID := r.Header.Get("X-Trace-Id")
		if traceID == "" {
			traceID = newTraceID()
		}
		ctx := WithTraceID(r.Context(), traceID)
		w.Header().Set("X-Trace-Id", traceID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// InstrumentedMux wraps an http.ServeMux with metrics and trace middleware.
func InstrumentedMux(mux http.Handler) http.Handler {
	return TraceMiddleware(MetricsMiddleware(mux))
}

func statusClass(code int) string {
	switch {
	case code < 200:
		return "1xx"
	case code < 300:
		return "2xx"
	case code < 400:
		return "3xx"
	case code < 500:
		return "4xx"
	default:
		return "5xx"
	}
}

// normalizePath replaces numeric path segments with :id to reduce cardinality.
// e.g. /api/orders/12345 → /api/orders/:id
func normalizePath(path string) string {
	if len(path) > 64 {
		path = path[:64]
	}
	// Walk segments; replace pure-numeric segments with :id.
	out := make([]byte, 0, len(path))
	i := 0
	for i < len(path) {
		// Copy the leading slash.
		if path[i] == '/' {
			out = append(out, '/')
			i++
			continue
		}
		// Find segment end.
		j := i
		for j < len(path) && path[j] != '/' {
			j++
		}
		seg := path[i:j]
		if isNumeric(seg) {
			out = append(out, ':')
			out = append(out, 'i', 'd')
		} else {
			out = append(out, seg...)
		}
		i = j
	}
	if len(out) == 0 {
		return "/"
	}
	return string(out)
}

func isNumeric(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
