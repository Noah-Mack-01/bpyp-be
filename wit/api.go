package wit

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

// ProcessMessage sends a message to Wit.ai for analysis
func ProcessMessage(message string) ([]byte, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%vmessage?q=%v", os.Getenv("BPYP_WIT_URL"), url.QueryEscape(message)), nil)
	if err != nil {
		log.Printf("Error on Creation of HTTP Request: %v", err)
		return nil, err
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", os.Getenv("BPYP_BEARER_API")))
	request.Header.Add("Accept", "*/*")

	resp, err := getClient().Do(request)
	if err != nil {
		log.Printf("Error on HTTP Request to wit.ai: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error on parsing response: %v", err)
		return nil, err
	}
	return body, err
}

func getClient() *http.Client {
	return &http.Client{}
}
