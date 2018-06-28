package userdata

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockMDClient struct{}
type mockBody struct {
	r io.Reader
}

func (m *mockBody) Close() error {
	return nil
}
func (m *mockBody) Read(b []byte) (int, error) {
	return m.r.Read(b)
}

func (m *mockMDClient) Get(url string) (*http.Response, error) {
	var body io.Reader
	if url == "http://169.254.42.42/user_data?format=json" {
		body = strings.NewReader(`{
			"user_data": [
				"ssh-host-fingerprints",
				"mykey"
			]
		}`)
	} else {
		body = strings.NewReader(`c`)
	}
	return &http.Response{
		Body: &mockBody{r: body},
	}, nil
}
func TestSelf(t *testing.T) {
	h := &mockMDClient{}
	md, err := Self(h)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(md))
	assert.Equal(t, "c", md["mykey"])
}
