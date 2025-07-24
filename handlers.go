package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	_ "github.com/lib/pq"
)

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

func (cfg *apiConfig) handlerUsers(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	data := struct {
		Email string `json:"email"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		log.Printf("failed to decode request: %v", err)
		w.WriteHeader(400)
		fmt.Fprintf(w, "{\"error\":\"Something went wrong\"}")
		return
	}
	user, err := cfg.db.CreateUser(req.Context(), data.Email)
	if err != nil {
		log.Printf("failed to create user: %v", err)
		w.WriteHeader(400)
		fmt.Fprintf(w, "{\"error\":\"Something went wrong\"}")
		return
	}

	w.WriteHeader(http.StatusCreated)
	fmt.Fprintf(w, "%v%v%v", "{\"id\":\"", user.ID, "\",")
	fmt.Fprintf(w, "%v%v%v", "\"created_at\":\"", user.CreatedAt, "\",")
	fmt.Fprintf(w, "%v%v%v", "\"updated_at\":\"", user.UpdatedAt, "\",")
	fmt.Fprintf(w, "%v%v%v", "\"email\":\"", user.Email, "\"}")
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
