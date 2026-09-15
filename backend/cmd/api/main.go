package main

import (
	"context"
	"log"
	"net/http"

	"os"
	"strings"

	"splitthebill/backend/internal/migration"
	"splitthebill/backend/internal/room"
	"splitthebill/backend/internal/store"
)

func main() {
	ctx := context.Background()

	appStore := createStore(ctx)
	roomHandler := room.NewHandler(appStore)

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

	log.Println(
		"SplitCheck API is running on",
		addr,
	)

	if err := http.ListenAndServe(
		addr,
		withCORS(mux),
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

	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)

		if origin == "" || origin == "*" {
			continue
		}

		allowedOrigins[origin] = true
	}

	return allowedOrigins
}
