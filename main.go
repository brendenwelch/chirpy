package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	code, err := fmt.Fprintf(w, "Hits: %d", cfg.fileserverHits.Load())
	if err != nil {
		fmt.Printf("failed to respond with code %v: %v", code, err)
	}
}

func (cfg *apiConfig) handlerResetMetrics(w http.ResponseWriter, req *http.Request) {
	cfg.fileserverHits.Store(0)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	code, err := fmt.Fprint(w, http.StatusText(http.StatusOK))
	if err != nil {
		fmt.Printf("failed to respond with code %v: %v", code, err)
	}
}

func handlerHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	code, err := fmt.Fprint(w, http.StatusText(http.StatusOK))
	if err != nil {
		fmt.Printf("failed to respond with code %v: %v", code, err)
	}
}

func main() {
	cfg := &apiConfig{}
	mux := http.NewServeMux()

	fs := http.StripPrefix("/app/", http.FileServer(http.Dir(".")))
	mux.Handle("/app/", cfg.middlewareMetricsInc(fs))

	mux.HandleFunc("/healthz", handlerHealth)
	mux.HandleFunc("/metrics", cfg.handlerMetrics)
	mux.HandleFunc("/reset", cfg.handlerResetMetrics)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server closed: %v", err)
	}
}
