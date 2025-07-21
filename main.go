package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	code, err := fmt.Fprintf(w, `<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
</html>`, cfg.fileserverHits.Load())
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

func handlerValidateChirp(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := struct {
		Body string `json:"body"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		log.Printf("failed to decode request: %v", err)
		w.WriteHeader(400)
		fmt.Fprintf(w, "{\"error\":\"Something went wrong\"}")
		return
	}

	if len(data.Body) > 140 {
		w.WriteHeader(400)
		fmt.Fprintf(w, "{\"error\":\"Chirp is too long\"}")
		return
	}

	profanes := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	words := strings.Split(data.Body, " ")
	for i, word := range words {
		_, exists := profanes[strings.ToLower(word)]
		if exists {
			words[i] = "****"
		}
	}
	cleaned := strings.Join(words, " ")

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "%v%v%v", "{\"cleaned_body\":\"", cleaned, "\"}")
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

	mux.HandleFunc("GET /api/healthz", handlerHealth)
	mux.HandleFunc("POST /api/validate_chirp", handlerValidateChirp)

	mux.HandleFunc("GET /admin/metrics", cfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", cfg.handlerResetMetrics)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server closed: %v", err)
	}
}
