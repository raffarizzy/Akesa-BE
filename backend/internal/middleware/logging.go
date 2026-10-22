package middleware

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rec *statusRecorder) WriteHeader(status int) {
	rec.status = status
	rec.ResponseWriter.WriteHeader(status)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	if rec.status >= 400 {
		rec.body.Write(b)
	}
	return rec.ResponseWriter.Write(b)
}

// Hijack implements http.Hijacker so that WebSocket connections can upgrade properly through this middleware.
func (rec *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hj, ok := rec.ResponseWriter.(http.Hijacker); ok {
		return hj.Hijack()
	}
	return nil, nil, fmt.Errorf("response writer does not implement http.Hijacker")
}

// Flush implements http.Flusher to support streaming responses.
func (rec *statusRecorder) Flush() {
	if fl, ok := rec.ResponseWriter.(http.Flusher); ok {
		fl.Flush()
	}
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		if rec.status >= 400 && rec.body.Len() > 0 {
			log.Printf(
				"%s %s -> %d (%s) | Error: %s",
				r.Method,
				r.URL.Path,
				rec.status,
				time.Since(start),
				strings.TrimSpace(rec.body.String()),
			)
		} else {
			log.Printf(
				"%s %s -> %d (%s)",
				r.Method,
				r.URL.Path,
				rec.status,
				time.Since(start),
			)
		}
	})
}
