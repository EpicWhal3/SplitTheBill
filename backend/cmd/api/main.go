package main

import (
	"log"
	"net/http"
	"splitcheck/backend/internal/room"
	"splitcheck/backend/internal/store"
)

func main() {
	memoryStore := store.NewMemoryStore()
	roomHandler := room.NewHandler(memoryStore)

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
