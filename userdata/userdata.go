package userdata

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

var retries = 5

type Userdata map[string]string

type useradataList struct {
	UserData []string `json:"user_data"`
}

type httpGetter interface {
	Get(url string) (*http.Response, error)
}

func getJSON(client httpGetter, url string, ans interface{}) error {
	retried := retries
	for retried > 0 {
		resp, err := client.Get(url)
		if err != nil {
			if resp.Body != nil {
				resp.Body.Close()
			}
			retried--
			time.Sleep(5 * time.Second)
			continue
		}
		err = json.NewDecoder(resp.Body).Decode(ans)
		if err != nil {
			resp.Body.Close()
			retried--
			time.Sleep(5 * time.Second)
			continue
		}
		return resp.Body.Close()
	}
	return errors.New("failed to fetch data: retried 5 times")
}
func getRawString(client httpGetter, url string) (string, error) {
	retried := retries
	for retried > 0 {
		resp, err := client.Get(url)
		if err != nil {
			retried--
			time.Sleep(5 * time.Second)
			continue
		}
		buff, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			resp.Body.Close()
			retried--
			time.Sleep(5 * time.Second)
			continue
		}
		resp.Body.Close()
		return string(buff), nil
	}
	return "", errors.New("failed to fetch data: retried 5 times")
}

func Self(client httpGetter) (Userdata, error) {
	data := Userdata{}
	list := useradataList{}
	err := getJSON(client, "http://169.254.42.42/user_data?format=json", &list)
	if err != nil {
		return nil, fmt.Errorf("failed to read key list: %v", err)
	}
	for _, elt := range list.UserData {
		item, err := getRawString(client, fmt.Sprintf("%s/%s", "http://169.254.42.42/user_data", elt))
		if err != nil {
			return nil, fmt.Errorf("failed to read key %s: %v", elt, err)
		}
		data[elt] = item
	}
	return data, nil
}
