package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/go-chi/chi/v5"
)

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8083"
	}

	var mu sync.Mutex
	received := make([]any, 0)

	r := chi.NewRouter()
	r.Post("/hook", func(w http.ResponseWriter, req *http.Request) {
		var row any
		if err := json.NewDecoder(req.Body).Decode(&row); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, row)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	r.Get("/received", func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(received)
	})

	log.Fatal(http.ListenAndServe(addr, r))
}
