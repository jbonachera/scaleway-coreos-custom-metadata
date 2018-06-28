package userdata

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

type Userdata map[string]string

type useradataList struct {
	UserData []string `json:"user_data"`
}

type httpGetter interface {
	Get(url string) (*http.Response, error)
}

func getJSON(client httpGetter, url string, ans interface{}) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(ans)
}
func getRawString(client httpGetter, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buff, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(buff), nil
}

func Self(client httpGetter) (Userdata, error) {
	data := Userdata{}
	list := useradataList{}
	err := getJSON(client, "http://169.254.42.42/user_data?format=json", &list)
	if err != nil {
		return nil, err
	}
	for _, elt := range list.UserData {
		item, err := getRawString(client, fmt.Sprintf("%s/%s", "http://169.254.42.42/user_data", elt))
		if err != nil {
			return nil, err
		}
		log.Println(item)
		data[elt] = item

	}
	return data, nil
}
