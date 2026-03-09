package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// GenerateAPIToken creates a new API token with the "gatie_" prefix.
// Returns the raw token (shown once to the user) and its SHA-256 hash (stored in DB).
// At auth time: SHA256(presented_token) is compared against the stored hash.
func GenerateAPIToken() (rawToken, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}
	rawToken = "gatie_" + hex.EncodeToString(b)
	h256 := sha256.Sum256([]byte(rawToken))
	hash = hex.EncodeToString(h256[:])
	return rawToken, hash, nil
}

// HashPassword hashes a plaintext password using bcrypt with the default cost.
func HashPassword(password string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// CheckPassword compares a plaintext password against a bcrypt hash.
func CheckPassword(hashedValue, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashedValue), []byte(password))
}
