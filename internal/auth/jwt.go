package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// GenerateToken creates a signed HS256 JWT with sub="admin" and 30-day expiry.
func GenerateToken(secret string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"sub": "admin",
		"iat": now.Unix(),
		"exp": now.Add(30 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

// ValidateToken parses and verifies the JWT. Returns true if valid.
func ValidateToken(tokenStr, secret string) (bool, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return false, err
	}
	return token.Valid, nil
}
