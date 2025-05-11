package repository

import (
	"fmt"
	"log"
	"os"

	"github.com/golang-jwt/jwt/v4"
	"github.com/supabase-community/supabase-go"
)

func getClient() *supabase.Client {
	client, err := supabase.NewClient(os.Getenv("BPYP_SUPABASE_URL"), os.Getenv("BPYP_SUPABASE_ANON_KEY"), &supabase.ClientOptions{})
	if err != nil {
		log.Fatalf("Cannot initialize client, %v", err)
	}
	return client
}

/*
	func GetUser(jwt string) (string, error) {
		client := getClient()

		response, _, err := getClient().From("sessions").Select("user_id", "exact", false).Eq("session_id", session).Single().Execute()
		if err != nil {
			return "", err
		}
		userID := string(response)
		return userID, nil
	}
*/

// If you want to fully verify the token as well:
func VerifyAndGetUserID(tokenString string) (string, error) {
	// Get Supabase JWT secret from environment
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
