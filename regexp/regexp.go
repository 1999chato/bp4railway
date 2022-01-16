package regexp

import (
	"encoding/json"
	"regexp"
)

func MustCompile(str string) *Regexp {
	return &Regexp{Regexp: regexp.MustCompile(str)}
}

type Regexp struct {
	*regexp.Regexp
}

func (r *Regexp) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Regexp.String())
}

func (r *Regexp) UnmarshalJSON(data []byte) (err error) {
	var value string
	err = json.Unmarshal(data, &value)
	if err != nil {
		return
	}
	r.Regexp, err = regexp.Compile(value)
	return
}
