package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
)

// test that a password is hashed correctly and is not empty

func TestHashedPassword(t *testing.T) {
	pwd := "pa$$word"
	hash, err := HashPassword(pwd)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}
	if hash == "" {
		t.Fatalf("Expected non-empty hash")
	}
	if hash == pwd {
		t.Fatal("hash should not equal plain password")
	}
}

// hash with HashPassword and then check with CheckPasswordHash
// the given password should return true for the check
// test that a wrong password fails the check
func TestCheckHashPassword(t *testing.T) {
	pwd := "AVerySt0ngPwd"
	hash, err := HashPassword(pwd)
	if err != nil {
		t.Fatalf("HashPassword returned error: %v", err)
	}

	ok, err := CheckPasswordHash(pwd, hash)
	if err != nil {
		t.Fatalf("CheckPasswordHash returned error: %v", err)
	}
	if !ok {
		t.Fatal("expected valid password to verify")
	}

	wrong, err := CheckPasswordHash("wrong", hash)
	if err == nil && wrong {
		t.Fatal("expected wrong password to fail")
	}
	_, err = CheckPasswordHash(pwd, "invalid-hash")
	if err == nil {
		t.Fatal("expected malformed hash to return an error")
	}
}

// test for MakeJWT function
func TestMakeJWT(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "AllForTheS@keOfIt"
	expiresIn := time.Millisecond * 1000 * 60
	token, err := MakeJWT(userID, tokenSecret, expiresIn)
	if err != nil {
		t.Fatalf("expected a well formed token: %v", err)
	}
	// test that not empty
	if token == "" {
		t.Fatalf("expected token to not be empty")
	}

	gotID, validateErr := ValidateJWT(token, tokenSecret)
	if validateErr != nil {
		t.Fatalf("Validate returned error: %v", validateErr)
	}

	// check token is correctly validated
	if gotID != userID {
		t.Fatalf("expected %v, got %v", userID, gotID)
	}
}

// create a JWT, then validate it
// use a wrong secret for a negative test
func TestValidateJWTExpiredToken(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "AllForTheS@keOfIt"
	expiresInPast := -time.Minute
	token, err := MakeJWT(userID, tokenSecret, expiresInPast)
	if err != nil {
		t.Fatalf("MakeJWT returned error: %v", err)
	}
	gotID, err := ValidateJWT(token, tokenSecret)
	if err == nil {
        t.Fatal("expected error for expired token")
    }
	if gotID != uuid.Nil {
		t.Fatal("expected nil UUID for expired token")
	}

}
func TestValidateJWTWrongSecret(t *testing.T) {
	userID := uuid.New()
	tokenSecret := "AllForTheS@keOfIt"
	wrongTokenSecret := "WrongForFakingIt"
	expiresInFuture := time.Minute
	token2, err := MakeJWT(userID, tokenSecret, expiresInFuture)
	if err != nil {
		t.Fatalf("MakeJWT returned an error: %v", err)
	}
	id, validateErr := ValidateJWT(token2, wrongTokenSecret)
	if validateErr == nil {
		t.Fatal("expected error for wrong secret")
	}
	if id != uuid.Nil {
		t.Fatal("expected nil UUID wrong secret token")
	}
}

func TestGetBearerToken(t *testing.T) {
	headers := http.Header{}
	headers.Set("Authorization", "Bearer my.jwt.token")
	token, err := GetBearerToken(headers)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if token != "my.jwt.token" {
		t.Fatalf("expected %q, got %q", "my.jwt.token", token)
	}

}

func TestGetBearerToken_NoHeader(t *testing.T) {
	headers := http.Header{}
	token, err := GetBearerToken((headers))
	
	if err == nil { // expecting an error here
		t.Fatal("expected error when authorization not present")
	}
	if token != "" { // expecting an error here
		t.Fatalf("expected empty token, got %q", token)
	}
}