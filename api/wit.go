package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

const WIT_GET_MESSAGE = "https://api.wit.ai/message"

func ProcessMessage(message *[]byte) ([]byte, error) {
	client := http.DefaultClient
	request, err := http.NewRequest("GET", fmt.Sprintf("%s?q=%s", WIT_GET_MESSAGE, url.QueryEscape(string(*message))), nil)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Authorization", fmt.Sprintf("Bearer %v", os.Getenv("WITAI_KEY")))
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("Accept", "*/*")
	request.Header.Add("User-Agent", "Go-http-client/1.1")

	res, err := httputil.DumpRequest(request, true)
	if err != nil {
		panic(err)
	}
	log.Printf("%v", string(res))
	response, err := client.Do(request)
	if err != nil && response.StatusCode == http.StatusOK {
		return nil, err
	}
	defer response.Body.Close()
	return io.ReadAll(response.Body)
}
