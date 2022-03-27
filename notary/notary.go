package notary

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

type Alg string

const NoneAlg = Alg("none")
const Ed25519Alg = Alg("ed25519")

var ErrVerifySign = errors.New("verify sign failed")
var ErrVerifyKeyNotFound = errors.New("verify key no found")
var ErrNoneSign = errors.New("not allow none sign")
var ErrShouldNoKey = errors.New("should no key")
var ErrNoSign = errors.New("sign no found")
var ErrUnkonwnAlgorithm = errors.New("unknown algorithm")

type State interface {
	GetKey(Domain string) (Algorithm Alg, Key []byte, err error)
	SetKey(Domain string, Algorithm Alg, Key []byte) (err error)
}

type Notarize struct {
	AllowNoneSign bool
	State         State
	Rand          io.Reader
}

func (n *Notarize) GenerateKey(Algorithm Alg) (VerifyKey, SignKey []byte, err error) {
	switch Algorithm {
	case Ed25519Alg:
		VerifyKey, SignKey, err = ed25519.GenerateKey(n.Rand)
		return
	case NoneAlg:
		err = ErrShouldNoKey
		return
	default:
		err = ErrUnkonwnAlgorithm
		return
	}
}

func (n *Notarize) GetVerifyKey(Domain string, Algorithm Alg) (key []byte, err error) {
	if n.State == nil {
		return
	}
	algorithm, key, err := n.State.GetKey(Domain)
	if err != nil {
		return
	}
	if key != nil && algorithm != Algorithm {
		err = ErrUnkonwnAlgorithm
	}
	return
}

func (n *Notarize) SetVerifyKey(Domain string, Algorithm Alg, key []byte) (err error) {
	if n.State == nil {
		return
	}
	return n.State.SetKey(Domain, Algorithm, key)
}

/*
Token = Header "." Payload "." Signature
	  | Header "." Payload

Payload = Base64(raw)

Header = Base64({"D":"","A":"","K":""})
D = Domain is optional, else use global domain.
A = Algorithm is optional, default is "ed25519". algoritm can't change if not none.
K = Key is optional, if set means upsert new verify key

Signature = Base64(sign[Algorithm](Header+"."+Payload, SignKey))
*/

func (n *Notarize) EncodeToken(Payload []byte, Domain string, Algorithm Alg, SignKey []byte, NewVerifyKey []byte) (token string, err error) {
	if Payload == nil {
		err = errors.New("payload is nil")
		return
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(Payload)

	switch Algorithm {
	case Ed25519Alg:
		if SignKey == nil {
			err = errors.New("sign key is nil")
			return
		}

		Header := map[string]string{"A": string(Ed25519Alg)}
		if Domain != "" {
			Header["D"] = Domain
		}
		if NewVerifyKey != nil {
			Header["K"] = base64.RawURLEncoding.EncodeToString(NewVerifyKey)
		}
		var header []byte
		header, err = json.Marshal(Header)
		if err != nil {
			return
		}
		encodedHeader := base64.RawURLEncoding.EncodeToString(header)

		token = encodedHeader + "." + encodedPayload
		sign := ed25519.Sign(SignKey, []byte(token))
		token += "." + base64.RawURLEncoding.EncodeToString(sign)

		return
	case NoneAlg:
		if SignKey != nil || NewVerifyKey != nil {
			err = ErrShouldNoKey
			return
		}
		token = base64.RawURLEncoding.EncodeToString([]byte(`{"A":"none"}`)) + "." + encodedPayload
		return
	default:
		err = ErrUnkonwnAlgorithm
		return
	}
}

func (n *Notarize) DecodeToken(token string) (Payload []byte, Domain string, Algorithm Alg, Sign []byte, NewVerifyKey []byte, VerifyKey []byte, err error) {
	segment := strings.Split(token, ".")
	if len(segment) < 2 {
		err = errors.New("token format error")
		return
	}

	encodedPayload := segment[1]
	Payload, err = base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		err = fmt.Errorf("token format error: %w", err)
		return
	}

	encodedHeader := segment[0]
	header, err := base64.RawURLEncoding.DecodeString(encodedHeader)
	if err != nil {
		err = fmt.Errorf("token format error: %w", err)
		return
	}
	var Header map[string]string
	err = json.Unmarshal(header, &Header)
	if err != nil {
		err = fmt.Errorf("token format error: %w", err)
		return
	}

	Algorithm = Alg(Header["A"])
	switch Algorithm {
	case NoneAlg:
		if !n.AllowNoneSign {
			err = ErrNoneSign
		}
		return
	case Ed25519Alg:
		if len(segment) < 3 {
			err = ErrNoSign
			return
		}

		encodedSign := segment[2]
		Sign, err = base64.RawURLEncoding.DecodeString(encodedSign)
		if err != nil {
			err = fmt.Errorf("sign format error: %w", err)
			return
		}

		body := []byte(encodedHeader + "." + encodedPayload)
		Domain = Header["D"]

		if key, ok := Header["K"]; ok {
			NewVerifyKey, err = base64.RawURLEncoding.DecodeString(key)
			if err != nil {
				err = fmt.Errorf("verify key format error: %w", err)
				return
			}
		}

		VerifyKey, err = n.GetVerifyKey(Domain, Algorithm)
		if err != nil {
			return
		}

		if VerifyKey == nil {
			if NewVerifyKey == nil {
				err = ErrVerifyKeyNotFound
				return
			} else {
				VerifyKey = NewVerifyKey
			}
		} else if bytes.Equal(NewVerifyKey, VerifyKey) {
			NewVerifyKey = nil
		}

		if !ed25519.Verify(VerifyKey, body, Sign) {
			err = ErrVerifySign
			return
		}

		if NewVerifyKey != nil {
			err = n.SetVerifyKey(Domain, Algorithm, NewVerifyKey)
			if err != nil {
				return
			}
		}
		return
	default:
		err = ErrUnkonwnAlgorithm
		return
	}
}
