package main

import (
	"encoding/json"
	"log"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultMaxRequestBodyBytes        int64 = 1048576
	defaultRateLimitRequestsPerSecond       = 10.0
	defaultRateLimitBurst                   = 20
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status != 0 {
		return
	}

	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(data []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	return r.ResponseWriter.Write(data)
}

func (r *statusRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func withRequestLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		startedAt := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}

		log.Printf(
			"%s %s %d %s %s",
			r.Method,
			r.URL.Path,
			status,
			time.Since(startedAt).Round(time.Microsecond),
			r.RemoteAddr,
		)
	})
}

func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")

		next.ServeHTTP(w, r)
	})
}

func withMaxRequestBody(
	maxBytes int64,
) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Body != nil {
				r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			}

			next.ServeHTTP(w, r)
		})
	}
}

type tokenBucket struct {
	tokens     float64
	lastRefill time.Time
}

type rateLimiter struct {
	mu            sync.Mutex
	buckets       map[string]*tokenBucket
	ratePerSecond float64
	burst         float64
}

func newRateLimiter(ratePerSecond float64, burst int) *rateLimiter {
	return &rateLimiter{
		buckets:       make(map[string]*tokenBucket),
		ratePerSecond: ratePerSecond,
		burst:         float64(burst),
	}
}

func (l *rateLimiter) allow(clientKey string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	bucket, ok := l.buckets[clientKey]
	if !ok {
		bucket = &tokenBucket{
			tokens:     l.burst,
			lastRefill: now,
		}
		l.buckets[clientKey] = bucket
	}

	elapsed := now.Sub(bucket.lastRefill).Seconds()
	if elapsed > 0 {
		bucket.tokens = math.Min(
			l.burst,
			bucket.tokens+elapsed*l.ratePerSecond,
		)
		bucket.lastRefill = now
	}

	if bucket.tokens < 1 {
		return false
	}

	bucket.tokens--
	return true
}

func withRateLimit(
	limiter *rateLimiter,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.allow(clientIP(r)) {
			writeMiddlewareError(
				w,
				http.StatusTooManyRequests,
				"rate limit exceeded",
			)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

func writeMiddlewareError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func resolveMaxRequestBodyBytes() int64 {
	raw := strings.TrimSpace(os.Getenv("MAX_REQUEST_BODY_BYTES"))
	if raw == "" {
		return defaultMaxRequestBodyBytes
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return defaultMaxRequestBodyBytes
	}

	return value
}

func resolveRateLimitConfig() (float64, int) {
	ratePerSecond := defaultRateLimitRequestsPerSecond
	burst := defaultRateLimitBurst

	rawRate := strings.TrimSpace(
		os.Getenv("RATE_LIMIT_REQUESTS_PER_SECOND"),
	)
	if rawRate != "" {
		if value, err := strconv.ParseFloat(rawRate, 64); err == nil && value > 0 {
			ratePerSecond = value
		}
	}

	rawBurst := strings.TrimSpace(os.Getenv("RATE_LIMIT_BURST"))
	if rawBurst != "" {
		if value, err := strconv.Atoi(rawBurst); err == nil && value > 0 {
			burst = value
		}
	}

	return ratePerSecond, burst
}
