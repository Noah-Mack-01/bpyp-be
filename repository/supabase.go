package repository

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/supabase-community/supabase-go"
	"noerkrieg.com/server/wit"
)

func getClient() *supabase.Client {

	// Create a client with the anonymous key first
	client, err := supabase.NewClient(os.Getenv("BPYP_SUPABASE_URL"), os.Getenv("BPYP_SUPABASE_SERVICE_KEY"), &supabase.ClientOptions{})
	if err != nil {
		log.Fatalf("Cannot initialize client, %v", err)
	}
	// Set the auth token to use the web_service role
	return client
}

func UploadExercises(exercises []wit.Exercise, userID string, message string) {
	client := getClient()
	for _, ex := range exercises {
		ex.UserId = userID
		ex.Summary = fmt.Sprintf(`"%v"`, message)
		ex.Timestamp = time.Now()
		if _, _, err := client.From("exercises").Insert(ex, true, "id", "minimal", "").Execute(); err != nil {
			log.Printf("Failed to insert exercise for job; %v", err)
		}
	}
}

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

func GetExercise(eid string) ([]byte, error) {
	client := getClient()
	data, _, err := client.From("exercises").Select("*", "1", false).Eq("id", eid).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to find exercise ID %v", eid)
	}

	log.Print(string(data))
	return data, nil
}
