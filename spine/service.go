package spine

import (
	"encoding/json"
	"errors"
	"io"
)

type Service interface {
	json.Marshaler
	json.Unmarshaler
	io.Closer
}

type Config interface {
	json.Marshaler
	json.Unmarshaler
}

type Status interface {
	json.Marshaler
}

var ErrServiceClosed = errors.New("service is closed")
var ErrServiceType = errors.New("service type error")
