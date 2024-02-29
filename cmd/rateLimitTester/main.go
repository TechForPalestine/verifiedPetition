package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

/**
 * This is a simple program that tries to spam the service.
 * It was easier to do this instead of writing a go test since I didn't
 * want to break the server into more files & pkgs.
 * This is also good for testing on a target server other than localhost if needed
 */

func createClientAndMakeRequest(urlStr string, email string, anonymize bool, wg *sync.WaitGroup) {
	// Create an HTTP client with a timeout of 10 seconds
	client := &http.Client{
		Timeout: time.Second * 10,
	}

	// Create form data
	formData := url.Values{
		"email":     {email},
		"anonymize": {fmt.Sprintf("%v", anonymize)},
	}

	// Create a POST request
	req, err := http.NewRequest("POST", urlStr, strings.NewReader(formData.Encode()))
	if err != nil {
		fmt.Println("Error creating the request:", err)
		return
	}

	// Set the content type to application/x-www-form-urlencoded
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending the request:", err)
		return
	}
	defer resp.Body.Close()

	// Print the response status code
	fmt.Println("Response status:", resp.Status)
	if wg != nil {
		wg.Done()
	}
}

func main() {
	_url := "http://localhost:8080/submit"
	email := "test@google.com"
	wg := new(sync.WaitGroup)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		createClientAndMakeRequest(_url, email, true, wg)
	}
	wg.Wait()
}
