package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"splitthebill/backend/internal/migration"
	"splitthebill/backend/internal/room"
	"splitthebill/backend/internal/store"
)

func main() {
	ctx := context.Background()

	appStore := createStore(ctx)
	roomHandler := room.NewHandler(appStore)
	startCleanupLoop(ctx, appStore)

	mux := http.NewServeMux()

	mux.Handle("/rooms", roomHandler)
	mux.Handle("/rooms/", roomHandler)

	mux.HandleFunc(
		"/health",
		func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		},
	)

	addr := ":" + resolvePort()

	rateLimitPerSecond, rateLimitBurst := resolveRateLimitConfig()

	handler := withCORS(mux)
	handler = withMaxRequestBody(resolveMaxRequestBodyBytes())(handler)
	handler = withRateLimit(
		newRateLimiter(rateLimitPerSecond, rateLimitBurst),
		handler,
	)
	handler = withSecurityHeaders(handler)
	handler = withRequestLogging(handler)

	log.Println(
		"SplitCheck API is running on",
		addr,
	)

	if err := http.ListenAndServe(
		addr,
		handler,
	); err != nil {
		log.Fatal(err)
	}
}

func resolvePort() string {
	port := strings.TrimSpace(os.Getenv("PORT"))

	if port == "" {
		return "8080"
	}

	return port
}

func createStore(
	ctx context.Context,
) store.Store {
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		log.Println(
			"DATABASE_URL is empty. Using in-memory store.",
		)

		return store.NewMemoryStore()
	}

	if err := migration.Run(databaseURL); err != nil {
		log.Fatal(
			"failed to run migrations:",
			err,
		)
	}

	postgresStore, err :=
		store.NewPostgresStore(
			ctx,
			databaseURL,
		)
	if err != nil {
		log.Fatal(
			"failed to connect to postgres:",
			err,
		)
	}

	log.Println("Connected to PostgreSQL.")

	return postgresStore
}

func withCORS(next http.Handler) http.Handler {
	allowedOrigins := resolveAllowedOrigins()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set(
			"Access-Control-Allow-Methods",
			"GET, POST, PUT, PATCH, DELETE, OPTIONS",
		)
		w.Header().Set(
			"Access-Control-Allow-Headers",
			"Content-Type, X-Admin-Token, X-Participant-Token",
		)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func resolveAllowedOrigins() map[string]bool {
	raw := strings.TrimSpace(os.Getenv("ALLOWED_ORIGINS"))

	if raw == "" {
		raw = "http://localhost:3000,http://127.0.0.1:3000,http://192.168.56.1:3000"
	}

	allowedOrigins := make(map[string]bool)

	for origin := range strings.SplitSeq(raw, ",") {
		origin = strings.TrimSpace(origin)

		if origin == "" || origin == "*" {
			continue
		}

		allowedOrigins[origin] = true
	}

	return allowedOrigins
}

func startCleanupLoop(ctx context.Context, appStore store.Store) {
	ttlHours := resolvePositiveInt("ROOM_TTL_HOURS", 168)
	intervalMinutes := resolvePositiveInt("ROOM_CLEANUP_INTERVAL_MINUTES", 60)

	ttl := time.Duration(ttlHours) * time.Hour
	interval := time.Duration(intervalMinutes) * time.Minute

	go func() {
		runCleanup := func() {
			olderThan := time.Now().UTC().Add(-ttl)
			removed, err := appStore.DeleteExpiredRooms(olderThan)
			if err != nil {
				log.Println("room cleanup failed:", err)
				return
			}
			if removed > 0 {
				log.Printf("removed %d expired rooms (TTL %s)", removed, ttl)
			}
		}

		runCleanup()
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runCleanup()
			}
		}
	}()
}

func resolvePositiveInt(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}
