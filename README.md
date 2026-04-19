# Chirpy Go Server

A small Go-based REST API backend for a Chirp-style application. It supports user creation, JWT authentication, refresh tokens, chirp creation, chirp querying, and admin metrics.

## What this project does

- Serves an HTTP API for creating users and posting chirps
- Stores data in PostgreSQL using generated SQL helpers from `sqlc`
- Uses JWTs for authentication and refresh tokens for session renewal
- Validates and sanitizes chirp text before saving it
- Provides admin endpoints for metrics and reset operations in development
- Serves static files under `/app` for frontend assets

## Why someone should care

This project is a useful starting point for building a lightweight Go backend with real-world features like:

- database-backed persistence with Postgres
- secure password hashing and token-based auth
- refresh token handling and token revocation
- request validation and JSON API responses
- clean separation between handlers, auth helpers, and generated data access code

It is also a practical example of combining Go standard libraries with `sqlc`, `goose`, and JWT-based auth in a simple service.

## How to install and run

### Prerequisites

- Go 1.26 or later
- PostgreSQL
- `sqlc` for generating database code (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
- `goose` for running migrations (`go install github.com/pressly/goose/v3/cmd/goose@latest`)

### Setup

1. Clone the repository:

   ```bash
   git clone https://github.com/abeonweb/chirpy-go-server.git
   cd chirpy-go-server
   ```

2. Install Go module dependencies:

   ```bash
   go mod download
   ```

3. Create the PostgreSQL database and run migrations:

   ```bash
   createdb gochirpy
   goose postgres "postgres://postgres:postgres@localhost:5432/gochirpy?sslmode=disable" up
   ```

4. Create a `.env` file in the project root with the database URL and runtime configuration:

   ```env
   DB_URL=postgres://postgres:postgres@localhost:5432/gochirpy?sslmode=disable
   PLATFORM=dev
   ```

5. Generate the SQL bindings with `sqlc`:
   ```bash
   sqlc generate
   ```

### Run the server

From the project root:

```bash
go run .
```

The server listens on `:8080` by default.

### Test the API

- `GET /api/healthz` — health check
- `POST /api/users` — create a new user
- `POST /api/chirps` — create a new chirp (requires Bearer token)

### Notes

- The `PLATFORM=dev` value enables the admin reset endpoint.
- Static files are served under `/app`.
- JWT secrets and refresh token handling are managed in the `internal/auth` package.
