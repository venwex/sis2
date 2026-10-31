package metrics

import (
	"context"
	"net/http"
	pb "pastebin/gen"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

var requests = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pastebin_requests_total",
		Help: "Total completed API requests.",
	},
	[]string{"protocol", "operation", "status"},
)

var duration = promauto.NewHistogramVec(
	prometheus.HistogramOpts{
		Name:    "pastebin_request_duration_seconds",
		Help:    "Server request handling duration in seconds.",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.2, 0.5, 1, 2, 3, 5},
	},
	[]string{"protocol", "operation"},
)

func observe(protocol string, operation string, result string, started time.Time) {
	requests.WithLabelValues(protocol, operation, result).Inc()
	duration.WithLabelValues(protocol, operation).Observe(time.Since(started).Seconds())
}

// Обёртка для получения HTTP-статуса ответа.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseWriter) WriteHeader(code int) {
	if w.wroteHeader {
		return
	}

	w.status = code
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}

	return w.ResponseWriter.Write(data)
}

func HTTP(operation string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next(wrapped, r)

		observe("rest", operation, strconv.Itoa(wrapped.status), started)
	}
}

func UnaryServerInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	operation := "other"

	switch info.FullMethod {
	case pb.PasteService_CreatePaste_FullMethodName:
		operation = "create"
	case pb.PasteService_GetPaste_FullMethodName:
		operation = "get"
	}

	started := time.Now()
	response, err := handler(ctx, req)

	observe("grpc", operation, status.Code(err).String(), started)

	return response, err
}
