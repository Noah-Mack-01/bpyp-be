package repository

import (
	"encoding/json"
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

func UploadExercises(exercises []wit.Exercise, userID string, message string) ([]byte, []error, error) {
	client := getClient()
	errors := make([]error, 0)
	compiled := make([]map[string]interface{}, 0)
	stats := struct {
		Total     int
		Succeeded int
		Failed    int
	}{
		Total: len(exercises),
	}

	// Set common fields on all exercises
	now := time.Now()
	for i := range exercises {
		exercises[i].UserId = userID
		exercises[i].Summary = fmt.Sprintf(`"%v"`, message)
		exercises[i].Timestamp = now
	}

	for _, ex := range exercises {
		// Attempt to upsert the exercise
		res, statusCode, err := client.From("exercises").Upsert(ex, "id", "representation", "exact").Execute()
		if err != nil {
			log.Printf("Failed to insert exercise %s: %v (status: %d)", ex.Exercise, err, statusCode)
			errors = append(errors, fmt.Errorf("failed to insert exercise %s: %w", ex.Exercise, err))
			stats.Failed++
			continue
		}

		// Process the response
		var js []map[string]interface{}
		if err := json.Unmarshal(res, &js); err != nil {
			nErr := fmt.Errorf("unmarshalling error for exercise %s: %w", ex.Exercise, err)
			errors = append(errors, nErr)
			stats.Failed++
			continue
		}

		if len(js) == 0 {
			nErr := fmt.Errorf("empty response for exercise %s", ex.Exercise)
			errors = append(errors, nErr)
			stats.Failed++
			continue
		}

		// Record success
		compiled = append(compiled, js[0])
		stats.Succeeded++
	}

	// Marshal results
	result, err := json.Marshal(compiled)
	if err != nil {
		return nil, errors, fmt.Errorf("failed to marshal compiled results: %w", err)
	}

	// Log operation summary
	log.Printf("Exercise upload summary: total=%d, succeeded=%d, failed=%d",
		stats.Total, stats.Succeeded, stats.Failed)
	if len(errors) > 0 {
		log.Printf("Upload errors: %v", errors)
	}

	return result, errors, nil
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

func GetExercise(eid string, uid string) ([]byte, error) {
	client := getClient()
	data, _, err := client.From("exercises").Select("*", "1", false).Eq("id", eid).Eq("user_id", uid).Execute()
	if err != nil {
		return nil, fmt.Errorf("failed to find exercise ID %v", eid)
	}

	log.Print(string(data))
	return data, nil
}
