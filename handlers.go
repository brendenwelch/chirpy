package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/brendenwelch/chirpy/internal/auth"
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
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&params); err != nil {
		respondWithError(w, 400, "Failed to decode request")
		return
	}

	hashed, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, 400, "Failed to hash password")
		return
	}

	user, err := cfg.db.CreateUser(req.Context(), database.CreateUserParams{
		Email:          params.Email,
		HashedPassword: hashed,
	})
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

func (cfg *apiConfig) handlerUpdateUser(w http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, 401, "No valid access token provided")
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithError(w, 401, "Invalid token: "+err.Error())
		return
	}

	params := struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&params); err != nil {
		respondWithError(w, 400, "Failed to decode request")
		return
	}
	hashedPassword, err := auth.HashPassword(params.Password)
	if err != nil {
		respondWithError(w, 400, "Failed to hash password")
		return
	}

	user, err := cfg.db.UpdateUser(req.Context(), database.UpdateUserParams{
		ID:             userID,
		Email:          params.Email,
		HashedPassword: hashedPassword,
	})
	if err != nil {
		respondWithError(w, 400, "Failed to update user")
		return
	}

	respondWithJSON(w, http.StatusOK, struct {
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

func (cfg *apiConfig) handlerLogin(w http.ResponseWriter, req *http.Request) {
	params := struct {
		Email            string `json:"email"`
		Password         string `json:"password"`
		ExpiresInSeconds int    `json:"expires_in_seconds,omitempty"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&params); err != nil {
		respondWithError(w, 400, "Failed to decode request")
		return
	}

	user, err := cfg.db.GetUserByEmail(req.Context(), params.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}
	if err := auth.CheckPasswordHash(params.Password, user.HashedPassword); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
		return
	}

	tokenDuration := time.Duration(params.ExpiresInSeconds)
	if tokenDuration < time.Second || tokenDuration > time.Hour {
		tokenDuration = time.Hour
	}
	token, err := auth.MakeJWT(user.ID, cfg.secret, tokenDuration)
	if err != nil {
		respondWithError(w, 400, "Failed to create access token")
		return
	}

	refreshTokenString, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(w, 400, "Failed to create refresh token")
		return
	}
	refreshToken, err := cfg.db.CreateRefreshToken(req.Context(), database.CreateRefreshTokenParams{
		Token:  refreshTokenString,
		UserID: user.ID,
	})
	if err != nil {
		respondWithError(w, 400, "Failed to add refresh token to database")
		return
	}

	respondWithJSON(w, http.StatusOK, struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
	}{
		ID:           user.ID,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
		Email:        user.Email,
		Token:        token,
		RefreshToken: refreshToken.Token,
	})
}

func (cfg *apiConfig) handlerRefresh(w http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, 400, "No valid token provided")
		return
	}
	refreshToken, err := cfg.db.GetRefreshToken(req.Context(), token)
	if err != nil {
		respondWithError(w, 401, "Refresh token not in database")
		return
	}
	if refreshToken.ExpiresAt.Before(time.Now()) {
		respondWithError(w, 401, "Refresh token expired")
		return
	}
	if refreshToken.RevokedAt.Valid {
		respondWithError(w, 401, "Refresh token revoked")
		return
	}

	accessToken, err := auth.MakeJWT(refreshToken.UserID, cfg.secret, time.Hour)
	if err != nil {
		respondWithError(w, 400, "Failed to create access token")
		return
	}

	respondWithJSON(w, http.StatusOK, struct {
		Token string `json:"token"`
	}{
		Token: accessToken,
	})
}

func (cfg *apiConfig) handlerRevoke(w http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, 400, "No valid token provided")
		return
	}
	err = cfg.db.RevokeRefreshToken(req.Context(), token)
	if err != nil {
		respondWithError(w, 400, "Refresh token does not exist")
	}

	respondWithJSON(w, 204, struct{}{})
}

func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "User not logged in")
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid user. Try logging in again: "+err.Error())
		return
	}

	params := struct {
		Body string `json:"body"`
	}{}
	if err := json.NewDecoder(req.Body).Decode(&params); err != nil {
		respondWithError(w, 400, "Failed to decode request")
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
	if len(cleaned) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	chirp, err := cfg.db.CreateChirp(req.Context(), database.CreateChirpParams{
		Body:   cleaned,
		UserID: userID,
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

func (cfg *apiConfig) handlerDeleteChirp(w http.ResponseWriter, req *http.Request) {
	token, err := auth.GetBearerToken(req.Header)
	if err != nil {
		respondWithError(w, 401, "No valid access token provided")
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.secret)
	if err != nil {
		respondWithError(w, 401, "Invalid token: "+err.Error())
		return
	}

	chirpID, err := uuid.Parse(req.PathValue("chirpID"))
	if err != nil {
		respondWithError(w, 400, "Invalid chirp ID")
		return
	}
	chirp, err := cfg.db.GetChirp(req.Context(), chirpID)
	if err != nil {
		respondWithError(w, 404, "Chirp not found")
		return
	}
	if userID != chirp.UserID {
		respondWithError(w, 403, "Not authorized to delete this chirp")
		return
	}

	err = cfg.db.DeleteChirp(req.Context(), chirp.ID)
	if err != nil {
		respondWithError(w, 400, "Failed to delete chirp")
		return
	}
	respondWithJSON(w, 204, struct{}{})
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
