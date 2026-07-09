package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"splitcheck/backend/internal/room"
	"splitcheck/backend/internal/store"
)

func main() {
	ctx := context.Background()
	appStore := createStore(ctx)
	roomHandler := room.NewHandler(appStore)

	mux := http.NewServeMux()
	mux.Handle("/rooms", roomHandler)
	mux.Handle("/rooms/", roomHandler)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := ":8080"
	log.Println("Starting server on", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func createStore(ctx context.Context) store.Store {
	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		log.Println("DATABASE_URL is empty. Using in-memory store")
		return store.NewMemoryStore()
	}

	postgresStore, err := store.NewPostgresStore(ctx, databaseURL)
	if err != nil {
		log.Fatal("failed to connect to postgres:", err)
	}

	log.Println("Connected to PostgreSQL")

	return postgresStore
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin == "http://localhost:3000" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}

		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
