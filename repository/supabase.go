package repository

import (
	"log"
	"os"

	"github.com/supabase-community/supabase-go"
)

func getClient() *supabase.Client {
	client, err := supabase.NewClient(os.Getenv("BPYP_SUPABASE_URL"), os.Getenv("BPYP_SUPABASE_ANON_KEY"), &supabase.ClientOptions{})
	if err != nil {
		log.Fatalf("Cannot initialize client, %v", err)
	}
	return client
}

func GetUser(session string) (string, error) {
	response, _, err := getClient().From("sessions").Select("user_id", "exact", false).Eq("session_id", session).Single().Execute()
	if err != nil {
		return "", err
	}
	userID := string(response)
	return userID, nil
}
