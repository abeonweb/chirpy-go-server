package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/abeonweb/chirpy-go-server/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db *database.Queries
	platform string
}


func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		return
	}
	dbQueries := database.New(db)

	mux := http.NewServeMux()
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	cfg := apiConfig{
		fileserverHits: atomic.Int32{},
		db: dbQueries,
		platform: platform,
	}
	
	mux.Handle("/app/", cfg.MiddlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, r *http.Request) {
		x := cfg.metricsHandler()
		str := fmt.Sprintf(`
		<html>
		<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
		</body>
		</html>`, x)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(200)
		_, err := w.Write([]byte(str))
		if err != nil {
			w.WriteHeader(500)
			return
		}
	})
	mux.HandleFunc("POST /admin/reset", cfg.resetHandler)

	mux.HandleFunc("GET /api/healthz", healthzHandler)
	mux.HandleFunc("POST /api/validate_chirp", handleValidateChirp)
	mux.HandleFunc("POST /api/users", cfg.handleCreateUser)
	server.ListenAndServe()
}

func (cfg *apiConfig) handleCreateUser(w http.ResponseWriter, r *http.Request){
	type parameters struct {
		Email string `json:"email"`
	}

	var params parameters
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&params)
	if err != nil {
		str := fmt.Sprintf("something went wrong: %v", err)
		respondWithError(w, 500, str)
		return
	}
	
	email :=  params.Email
	u, err := cfg.db.CreateUser(r.Context(), email)
	if err != nil {
		str := fmt.Sprintf("something went wrong: %v", err)
		respondWithError(w, 500, str)
		return
	}
	type User struct {
		ID uuid.UUID `json:"id"`
		Created_at time.Time `json:"created_at"`
		Updated_at time.Time `json:"updated_at"`
		Email string `json:"email"`
	}
	userJson := User{
		ID: u.ID,
		Created_at: u.CreatedAt,
		Updated_at: u.UpdatedAt,
		Email: u.Email,
	}
	data, err := json.Marshal(userJson)
	if err != nil{
		respondWithError(w, 500, "something went wrong")
		return
	}
	respondWithJSON(w, 201, data)

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
	
	type payload struct{
		message string
	}

	p := payload{ message:"Users deleted" }
	
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

func handleValidateChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	toValidate := parameters{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&toValidate)
	if err != nil {
		w.WriteHeader(500)
		return
	}

	MaxCharLen := 140

	type validData struct {
		Cleaned_body string `json:"cleaned_body"`
	}

	w.Header().Set("Content-Type", "application/json")
	
	if len(toValidate.Body) > MaxCharLen {
		 respondWithError(w, 400, "chirp is too long")
		 return
	}
	data := validData{
		Cleaned_body: profanityFixer(toValidate.Body),
	}
	respondWithJSON(w, 200, data)
	
}

func respondWithError(w http.ResponseWriter, code int, msg string){
	type errorData struct {
		Error string `json:"error"`
	}

	dataErr := errorData{
		Error: msg,
	}
	data, err := json.Marshal(dataErr)
	if err != nil {
		w.WriteHeader(500)
		return
		}	
		w.WriteHeader(code)
		w.Write(data)
}
	
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}){
		w.WriteHeader(code)
		dataJson, err := json.Marshal(payload)
		if err != nil {
			w.WriteHeader(500)
		}
		w.Write(dataJson)
}

func profanityFixer(chirp string) string {
	badWords := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}
	toReplace := strings.Split(chirp, " ")
	for _,swear := range badWords {
		for i, word := range toReplace{
			if swear == strings.ToLower(word){
				toReplace[i] = "****"
			}
		}
	}
	return strings.Join(toReplace, " ")
}
