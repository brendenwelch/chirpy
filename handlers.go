package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/brendenwelch/chirpy/internal/database"
	"github.com/google/uuid"
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
	fmt.Fprintf(w, `<html>
	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
</html>`, cfg.fileserverHits.Load())
}

func (cfg *apiConfig) handlerReset(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, http.StatusText(http.StatusForbidden))
		return
	}

	cfg.fileserverHits.Store(0)
	cfg.db.ResetUsers(req.Context())
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, http.StatusText(http.StatusOK))
}

func (cfg *apiConfig) handlerUsers(w http.ResponseWriter, req *http.Request) {
	params := struct {
		Email string `json:"email"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&params); err != nil {
		respondWithError(w, 400, "Failed to decode request")
		return
	}
	user, err := cfg.db.CreateUser(req.Context(), params.Email)
	if err != nil {
		respondWithError(w, 400, "Failed to create user")
		return
	}

	respondWithJSON(w, http.StatusCreated, struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Email     string    `json:"email"`
	}{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, req *http.Request) {
	params := struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&params); err != nil {
		respondWithError(w, 400, "Failed to decode request")
		return
	}

	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	profanes := map[string]struct{}{
		"kerfuffle": {},
		"sharbert":  {},
		"fornax":    {},
	}
	words := strings.Split(params.Body, " ")
	for i, word := range words {
		_, exists := profanes[strings.ToLower(word)]
		if exists {
			words[i] = "****"
		}
	}
	cleaned := strings.Join(words, " ")

	chirp, err := cfg.db.CreateChirp(req.Context(), database.CreateChirpParams{
		Body:   cleaned,
		UserID: params.UserID,
	})
	if err != nil {
		respondWithError(w, 400, "Failed to create chirp")
		return
	}

	respondWithJSON(w, http.StatusCreated, struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

func (cfg *apiConfig) handlerGetChirps(w http.ResponseWriter, req *http.Request) {
	chirps, err := cfg.db.GetAllChirps(req.Context())
	if err != nil {
		respondWithError(w, 400, "Failed to get chirps")
		return
	}

	type Chirp struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}
	var payload []Chirp
	for _, chirp := range chirps {
		payload = append(payload, Chirp{
			ID:        chirp.ID,
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID,
		})
	}
	respondWithJSON(w, http.StatusOK, payload)
}

func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, req *http.Request) {
	id, err := uuid.Parse(req.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, 400, "Invalid chirp ID")
		return
	}
	chirp, err := cfg.db.GetChirp(req.Context(), id)
	if err != nil {
		respondWithError(w, 404, "Failed to get chirp")
		return
	}
	respondWithJSON(w, http.StatusOK, struct {
		ID        uuid.UUID `json:"id"`
		CreatedAt time.Time `json:"created_at"`
		UpdatedAt time.Time `json:"updated_at"`
		Body      string    `json:"body"`
		UserID    uuid.UUID `json:"user_id"`
	}{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	})
}

func handlerHealth(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, http.StatusText(http.StatusOK))
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	log.Println(msg)
	respondWithJSON(w, code, struct {
		Error string `json:"error"`
	}{
		Error: msg,
	})
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(payload)
	if err != nil {
		respondWithError(w, 400, "Failed to marshal JSON")
		return
	}
	w.WriteHeader(code)
	fmt.Fprint(w, string(data))
}
