package state

import (
	"bytes"
	"net/http"
)

const stateURL = "http://169.254.42.42/state"

type HttpClient interface {
	Do(*http.Request) (*http.Response, error)
}

func SignalBooted(client HttpClient) error {
	payload := bytes.NewReader([]byte(`{"state_detail": "booted"}`))
	req, err := http.NewRequest("PATCH", stateURL, payload)
	if err != nil {
		return err
	}
	_, err = client.Do(req)
	return err
}
