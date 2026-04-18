package main

import (
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/abeonweb/chirpy-go-server/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	jwtSecret      string
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("JWT_SECRET")
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
		db:             dbQueries,
		platform:       platform,
		jwtSecret:      secret,
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
	mux.HandleFunc("POST /api/users", cfg.handleCreateUser)
	mux.HandleFunc("GET /api/chirps", cfg.handleGetAllChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", cfg.handleGetChirpByID)
	mux.HandleFunc("DELETE /api/chirps/{chirpID}", cfg.handleDeleteChirpByID)
	mux.HandleFunc("POST /api/chirps", cfg.handleCreateChirp)
	mux.HandleFunc("POST /api/login", cfg.handleLogin)
	mux.HandleFunc("POST /api/refresh", cfg.handleRefresh)
	mux.HandleFunc("POST /api/revoke", cfg.handleRevoke)
	mux.HandleFunc("PUT /api/users", cfg.handleLoginUpdate)
	mux.HandleFunc("POST /api/polka/webhooks", cfg.handleChirpyUpgrade)
	server.ListenAndServe()
}
