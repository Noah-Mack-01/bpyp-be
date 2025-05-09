package wit

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
)

// TODO: We have to set this up to asynchronously process these against the wit api.
//
//	First thought is to send "messages" aka workout logs to a kafka topic, then have multiple
//	worker instances running a main method in this package to call Wit, perform post-processing, and
//	transacting against the postgres back-end.
func ProcessMessage(message string) ([]byte, error) {
	request, err := http.NewRequest("GET", fmt.Sprintf("%vmessage?q=%v", os.Getenv("BPYP_WIT_URL"), url.QueryEscape(message)), nil)
	if err != nil {
		log.Fatalf("Error on Creation of HTTP Request: %v", err)
	}

	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", os.Getenv("BPYP_BEARER_API")))
	request.Header.Add("Accept", "*/*")

	log.Printf("%v", request)
	resp, err := getClient().Do(request)
	if err != nil {
		log.Fatalf("Error on HTTP Request to wit.ai: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error on parsing response: %v", err)
	}
	return body, err
}

func getClient() *http.Client {
	return &http.Client{}
}
