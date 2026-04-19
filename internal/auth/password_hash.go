package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {

	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", fmt.Errorf("Hashing error: %v", err)
	}
	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	isCorrect, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, fmt.Errorf("Check password error: %v", err)
	}
	return isCorrect, nil
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {

	type MyClaims struct {
		jwt.RegisteredClaims
	}
	claims := MyClaims{
		jwt.RegisteredClaims{
			Issuer:    "chirpy-access",
			Subject:   userID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Second * expiresIn)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	ss, err := token.SignedString([]byte(tokenSecret))

	return ss, err
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	type MyClaims struct {
		jwt.RegisteredClaims
	}

	token, err := jwt.ParseWithClaims(tokenString, &MyClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	}
	claims, ok := token.Claims.(*MyClaims)
	id, err := uuid.Parse(claims.Subject)
	if ok {
		return id, nil
	}

	return uuid.Nil, err
}

func GetBearerToken(headers http.Header) (string, error) {
	key := headers.Get("Authorization")
	if key == "" {
		return key, errors.New("No authorization header found")
	}
	token := strings.Replace(key, "Bearer ", "", 1)
	return token, nil
}

// generates a random 256-bit (32-byte) hex-encoded string
func MakeRefreshToken() string {
	key := make([]byte, 32)
	rand.Read(key)
	return hex.EncodeToString(key)
}

func GetAPIKey(headers http.Header) (string, error){
	authorization := headers.Get("Authorization")
	if authorization == "" {
		return "", errors.New("No authorization header found")
	}	
	key := strings.Replace(authorization, "ApiKey ", "", 1)
	if key == "" {
		return "", errors.New("No API key found")
	}	
	return key, nil
}