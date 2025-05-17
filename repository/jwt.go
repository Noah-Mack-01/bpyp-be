package repository

import (
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v4"
)

// If you want to fully verify the token as well:
func VerifyAndGetUserID(tokenString string) (string, error) {
	jwtSecret := os.Getenv("BPYP_POSTGRES_JWT_SECRET")
	if jwtSecret == "" {
		return "", fmt.Errorf("JWT Secret not set")
	}

	// Parse and verify the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Verify signing method is what you expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(jwtSecret), nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to verify token: %w", err)
	}

	// Check if token is valid
	if !token.Valid {
		return "", fmt.Errorf("invalid token")
	}

	// Extract user ID from claims
	if claims, ok := token.Claims.(jwt.MapClaims); ok {
		if sub, ok := claims["sub"].(string); ok {
			return sub, nil
		}
		return "", fmt.Errorf("sub claim not found or not a string")
	}

	return "", fmt.Errorf("invalid token claims")
}
