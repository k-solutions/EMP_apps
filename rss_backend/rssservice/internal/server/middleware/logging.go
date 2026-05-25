package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"
)

const LogFieldsKey contextKey = "log_fields"

type LogFields struct {
	JobID string
}

type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	if !r.written {
		r.statusCode = statusCode
		r.written = true
	}
	r.ResponseWriter.WriteHeader(statusCode)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if !r.written {
		r.statusCode = http.StatusOK
		r.written = true
	}
	return r.ResponseWriter.Write(b)
}

func Logging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			startTime := time.Now()

			fields := &LogFields{}
			ctx := context.WithValue(r.Context(), LogFieldsKey, fields)

			rec := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(rec, r.WithContext(ctx))

			durationMs := time.Since(startTime).Milliseconds()

			// Extract IP
			ip := r.RemoteAddr
			if forward := r.Header.Get("X-Forwarded-For"); forward != "" {
				ip = forward
			}

			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.statusCode,
				"duration_ms", durationMs,
				"job_id", fields.JobID,
				"remote_addr", ip,
			)
		})
	}
}

// SetLogJobID registers a job ID to be printed in the request's final structured log.
func SetLogJobID(ctx context.Context, jobID string) {
	if fields, ok := ctx.Value(LogFieldsKey).(*LogFields); ok {
		fields.JobID = jobID
	}
}
