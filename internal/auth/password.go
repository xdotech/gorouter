package auth

import (
	"golang.org/x/crypto/bcrypt"
)

// HashPassword returns a bcrypt hash of the plaintext password.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyPassword compares a plaintext password against a bcrypt hash.
func VerifyPassword(plain, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// VerifyInitialPassword compares a plaintext password against the raw INITIAL_PASSWORD value.
func VerifyInitialPassword(plain, initialPassword string) bool {
	return plain == initialPassword
}
