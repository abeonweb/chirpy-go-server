package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/abeonweb/chirpy-go-server/internal/auth"
	"github.com/abeonweb/chirpy-go-server/internal/database"
	"github.com/google/uuid"
)

type validChirpData struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

const ExpiresIn = time.Second * 60 * 60 // create JWT with 1 hour expiry constant

func (cfg *apiConfig) handleChirpyUpgrade(w http.ResponseWriter, r *http.Request) {

	polka := struct {
		Event string `json:"event"`
		Data  struct {
			UserID uuid.UUID `json:"user_id"`
		} `json:"data"`
	}{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&polka); err != nil {
		respondWithError(w, 404, fmt.Sprintf("%v", err))
		return
	}
	if polka.Event != "user.upgraded" {
		respondWithJSON(w, 204, "not user upgraded event")
		return
	}

	id := polka.Data.UserID
	user, dbErr := cfg.db.UpdateChirpyRedByID(r.Context(), id)
	if dbErr != nil || !user.IsChirpyRed {
		respondWithError(w, 404, fmt.Sprintf("%v", dbErr))
		return
	}
	
	respondWithJSON(w, 204, "user upgrade success")
	
}

func (cfg *apiConfig) handleLoginUpdate(w http.ResponseWriter, r *http.Request) {
	token, tokenErr := auth.GetBearerToken(r.Header)
	if tokenErr != nil {
		respondWithError(w, 401, fmt.Sprintf("authorization error: %v", tokenErr))
		return
	}

	id, validationErr := auth.ValidateJWT(token, cfg.jwtSecret)
	if validationErr != nil {
		respondWithError(w, 401, fmt.Sprintf("%v", validationErr))
		return
	}
	var params loginParameters
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&params); err != nil {
		respondWithError(w, 401, fmt.Sprintf("%v", err))
		return
	}
	hashedPwd, hashErr := auth.HashPassword(params.Password)
	if hashErr != nil {
		respondWithError(w, 401, fmt.Sprintf("%v", hashErr))
		return
	}
	data := database.UpdateUserLoginParams{
		ID:             id,
		Email:          params.Email,
		HashedPassword: hashedPwd,
	}

	dbData, dbErr := cfg.db.UpdateUserLogin(r.Context(), data)
	if dbErr != nil {
		respondWithError(w, 401, fmt.Sprintf("%v", dbErr))
		return
	}
	user := User{
		ID:          dbData.ID,
		CreatedAt:   dbData.CreatedAt,
		UpdatedAt:   dbData.UpdatedAt,
		Email:       dbData.Email,
		IsChirpyRed: dbData.IsChirpyRed,
	}
	respondWithJSON(w, 200, user)
}

func (cfg *apiConfig) handleRevoke(w http.ResponseWriter, r *http.Request) {
	token, tokenErr := auth.GetBearerToken(r.Header)
	if tokenErr != nil {
		respondWithError(w, 401, "authorization not found")
		return
	}

	data, err := cfg.db.UpdateRevokedAt(r.Context(), token)
	if err != nil || !data.RevokedAt.Valid {
		respondWithError(w, 401, "could not revoke token")
		return
	}

	respondWithJSON(w, 204, "token revoked successfully")
}

func (cfg *apiConfig) handleRefresh(w http.ResponseWriter, r *http.Request) {
	token, tokenErr := auth.GetBearerToken(r.Header)
	if tokenErr != nil {
		respondWithError(w, 401, "authorization not found")
		return
	}

	data, err := cfg.db.GetUserFromRefreshToken(r.Context(), token)

	canRevoke := data.RevokedAt.Valid
	if err != nil || time.Now().After(data.ExpiresAt) || canRevoke {
		respondWithError(w, 401, "Token has expired")
		return
	}
	userID := data.UserID
	accessToken, err := auth.MakeJWT(userID, cfg.jwtSecret, ExpiresIn)
	if err != nil {
		respondWithError(w, 401, fmt.Sprintf("%v", err))
		return
	}
	response := struct {
		Token string `json:"token"`
	}{
		Token: accessToken,
	}
	respondWithJSON(w, 200, response)
}

func (cfg *apiConfig) handleLogin(w http.ResponseWriter, r *http.Request) {

	var params loginParameters
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&params); err != nil {
		respondWithError(w, 500, "Incorrect email or password")
		return
	}

	u, getUserErr := cfg.db.GetUserByEmail(r.Context(), params.Email)
	if getUserErr != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}
	ok, pwdErr := auth.CheckPasswordHash(params.Password, u.HashedPassword)
	if !ok || pwdErr != nil {
		respondWithError(w, 401, "Incorrect email or password")
		return
	}

	// create JWT with 1 hour expiry
	accessToken, tokenErr := auth.MakeJWT(u.ID, cfg.jwtSecret, ExpiresIn)
	if tokenErr != nil {
		respondWithError(w, 500, fmt.Sprintf("something went wrong: %v", tokenErr))
	}
	// generate refresh token
	refresh := auth.MakeRefreshToken()
	// add refresh token to DB
	refreshTokenData := database.CreateRefreshTokenParams{
		Token:     refresh,
		UserID:    u.ID,
		ExpiresAt: time.Now().Add(time.Second * 60 * 60 * 24 * 60),
	}
	refreshDBData, refreshDBErr := cfg.db.CreateRefreshToken(r.Context(), refreshTokenData)
	if refreshDBErr != nil {
		respondWithError(w, 401, fmt.Sprintf("%v", refreshDBErr))
		return
	}

	// user without password
	user := struct {
		ID           uuid.UUID `json:"id"`
		CreatedAt    time.Time `json:"created_at"`
		UpdatedAt    time.Time `json:"updated_at"`
		Email        string    `json:"email"`
		Token        string    `json:"token"`
		RefreshToken string    `json:"refresh_token"`
		IsChirpyRed  bool      `json:"is_chirpy_red"`
	}{
		ID:           u.ID,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
		Email:        u.Email,
		Token:        accessToken,
		RefreshToken: refreshDBData.Token,
		IsChirpyRed:  u.IsChirpyRed,
	}
	respondWithJSON(w, 200, user)
}

func (cfg *apiConfig) handleGetChirpByID(w http.ResponseWriter, r *http.Request) {

	chirpStr := r.PathValue("chirpID")
	chirpID, parseErr := uuid.Parse(chirpStr)
	if parseErr != nil {
		respondWithError(w, 500, "invalid chirp ID")
		return
	}
	dbChirp, dbErr := cfg.db.GetChirpByID(r.Context(), chirpID)
	if dbErr != nil {
		respondWithError(w, 404, "chirp not found")
		return
	}
	chirp := validChirpData{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}
	respondWithJSON(w, 200, chirp)

}

func (cfg *apiConfig) handleDeleteChirpByID(w http.ResponseWriter, r *http.Request) {

	// validate user
	token, tokenErr := auth.GetBearerToken(r.Header)
	if tokenErr != nil {
		respondWithError(w, 401, fmt.Sprintf("%v", tokenErr))
		return
	}
	userID, validationErr := auth.ValidateJWT(token, cfg.jwtSecret)
	if validationErr != nil {
		respondWithError(w, 401, fmt.Sprintf("%v", tokenErr))
		return
	}

	chirpStr := r.PathValue("chirpID")
	chirpID, parseErr := uuid.Parse(chirpStr)
	if parseErr != nil {
		respondWithError(w, 500, "invalid chirp ID")
		return
	}

	// get chirp to confirm userID match
	dbChirp, dbErr := cfg.db.GetChirpByID(r.Context(), chirpID)
	if dbErr != nil {
		respondWithError(w, 404, "chirp not found")
		return
	}
	if dbChirp.UserID != userID {
		respondWithError(w, 403, "Not authorized to delete")
		return
	}
	deleteErr := cfg.db.DeleteChirpByID(r.Context(), chirpID)
	if deleteErr != nil {
		respondWithError(w, 500, fmt.Sprintf("%v", deleteErr))
		return
	}

	respondWithJSON(w, 204, "chirp deleted")

}

func (cfg *apiConfig) handleGetAllChirps(w http.ResponseWriter, r *http.Request) {
	dbChirps, chirpErr := cfg.db.GetAllChirps(r.Context())
	if chirpErr != nil {
		respondWithError(w, 500, "error retrieving chirps")
		return
	}
	chirps := []validChirpData{}
	for _, dbChirp := range dbChirps {
		chirps = append(chirps, validChirpData{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		})

	}
	respondWithJSON(w, 200, chirps)
}

func (cfg *apiConfig) handleCreateChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	toValidate := parameters{}
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&toValidate); err != nil {
		w.WriteHeader(500)
		return
	}
	// get authorization header
	token, tokenErr := auth.GetBearerToken(r.Header)
	if tokenErr != nil {
		str := fmt.Sprintf("something went wrong: %v", tokenErr)
		respondWithError(w, 500, str)
		return
	}
	// validate JWT
	validatedID, validateErr := auth.ValidateJWT(token, cfg.jwtSecret)
	if validateErr != nil {
		respondWithError(w, 401, fmt.Sprintf("something went wrong: %v", validateErr))
		return
	}

	MaxCharLen := 140

	if len(toValidate.Body) > MaxCharLen {
		respondWithError(w, 400, "chirp is too long")
		return
	}
	data := database.AddChirpParams{
		Body:   profanityFixer(toValidate.Body),
		UserID: validatedID,
	}
	chirp, chirpErr := cfg.db.AddChirp(r.Context(), data)
	if chirpErr != nil {
		str := fmt.Sprintf("something went wrong while adding your chirp: %v", chirpErr)
		respondWithError(w, 500, str)
		return
	}

	chirpData := validChirpData{
		ID:        chirp.ID,
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID,
	}
	respondWithJSON(w, 201, chirpData)

}

type loginParameters struct {
	Password string `json:"password"`
	Email    string `json:"email"`
}
type User struct {
	ID          uuid.UUID `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

func (cfg *apiConfig) handleCreateUser(w http.ResponseWriter, r *http.Request) {

	var params loginParameters
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		str := fmt.Sprintf("something went wrong: %v", err)
		respondWithError(w, 500, str)
		return
	}

	email := params.Email
	hashedPwd, pwdErr := auth.HashPassword(params.Password)
	if pwdErr != nil {
		respondWithError(w, 500, "Password not properly formed")
		return
	}

	userParams := database.CreateUserParams{
		Email:          email,
		HashedPassword: hashedPwd,
	}

	u, err := cfg.db.CreateUser(r.Context(), userParams)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("something went wrong: %v", err))
		return
	}

	userJson := User{
		ID:          u.ID,
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
		Email:       u.Email,
		IsChirpyRed: u.IsChirpyRed,
	}
	respondWithJSON(w, 201, userJson)
}

func (cfg *apiConfig) MiddlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var delta int32 = 1
		cfg.fileserverHits.Add(delta)
		next.ServeHTTP(w, r)

	})
}

func (cfg *apiConfig) metricsHandler() int32 {
	return cfg.fileserverHits.Load()
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, 403, "Forbidden")
		return
	}
	cfg.db.DeleteAllUsers(r.Context())

	type payload struct {
		message string
	}

	p := payload{message: "Users deleted"}

	respondWithJSON(w, 200, p)
}

func healthzHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	_, err := w.Write([]byte("OK"))
	if err != nil {
		w.WriteHeader(500)
		return
	}
}

func respondWithError(w http.ResponseWriter, code int, msg string) {

	dataErr := struct {
		Error string `json:"error"`
	}{
		Error: msg,
	}

	data, err := json.Marshal(dataErr)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.WriteHeader(code)
	w.Write(data)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	data, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(data)
}

func profanityFixer(chirp string) string {
	badWords := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}
	toReplace := strings.Split(chirp, " ")
	for _, swear := range badWords {
		for i, word := range toReplace {
			if swear == strings.ToLower(word) {
				toReplace[i] = "****"
			}
		}
	}
	return strings.Join(toReplace, " ")
}
