package url

import (
	"encoding/json"
	"net/url"
	"regexp"
)

type URL struct {
	url.URL
}

func (u *URL) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.URL.String())
}

func (u *URL) UnmarshalJSON(data []byte) (err error) {
	var v string
	err = json.Unmarshal(data, &v)
	if err != nil {
		return
	}
	return u.URL.UnmarshalBinary([]byte(v))
}

func (u *URL) AssertSchema(pattern ...string) error {
	return nil
}

func (u *URL) AssertHost(pattern ...string) error {
	return nil
}

func (u *URL) AssertRegexpHost(pattern ...*regexp.Regexp) error {
	return nil
}
