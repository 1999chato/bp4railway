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
	"sync"
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

type LocalState sync.Map

type key struct {
	Algorithm Alg
	Key       []byte
}

func (s *LocalState) GetKey(Domain string) (Algorithm Alg, Key []byte, err error) {
	m := (*sync.Map)(s)
	v, ok := m.Load(Domain)
	if !ok {
		return
	}

	k, ok := v.(*key)
	if !ok {
		err = errors.New("key type error")
		return
	}

	Algorithm = k.Algorithm
	Key = k.Key
	return
}

func (s *LocalState) SetKey(Domain string, Algorithm Alg, Key []byte) (err error) {
	m := (*sync.Map)(s)
	m.Store(Domain, &key{Algorithm, Key})
	return
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
	return n.State.SetKey(Domain, Algorithm, key)
}

/*
Token = Type "." Payload "." Signature
	  | Type "." Payload

Payload = Base64(raw)

Type = Base64({"D":"","A":"","K":""})
D = Domain is optional, else use global domain.
A = Algorithm is optional, default is "ed25519". algoritm can't change if not none.
K = Key is optional, if set means upsert new verify key

Signature = Base64(sign[Algorithm](Type+"."+Payload, SignKey))
*/

func (n *Notarize) GenerateToken(payload []byte, Domain string, Algorithm Alg, SignKey []byte, NewVerifyKey []byte) (token string, err error) {
	if payload == nil {
		err = errors.New("payload is nil")
		return
	}
	encodedPayload := base64.RawURLEncoding.EncodeToString(payload)

	switch Algorithm {
	case Ed25519Alg:
		if SignKey == nil {
			err = errors.New("sign key is nil")
			return
		}

		header := map[string]string{"A": string(Ed25519Alg)}
		if Domain != "" {
			header["D"] = Domain
		}
		if NewVerifyKey != nil {
			header["K"] = base64.RawURLEncoding.EncodeToString(NewVerifyKey)
		}
		var Header []byte
		Header, err = json.Marshal(header)
		if err != nil {
			return
		}
		encodedHeader := base64.RawURLEncoding.EncodeToString(Header)

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

func (n *Notarize) GetPayloadFromToken(token string) (payload []byte, err error) {
	segment := strings.Split(token, ".")
	if len(segment) < 2 {
		err = errors.New("token format error")
		return
	}

	encodedPayload := segment[1]
	payload, err = base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil {
		err = fmt.Errorf("token format error: %w", err)
		return
	}

	encodedHeader := segment[0]
	Header, err := base64.RawURLEncoding.DecodeString(encodedHeader)
	if err != nil {
		err = fmt.Errorf("token format error: %w", err)
		return
	}
	var header map[string]string
	err = json.Unmarshal(Header, &header)
	if err != nil {
		err = fmt.Errorf("token format error: %w", err)
		return
	}

	Algorithm := Alg(header["A"])
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
		var sign []byte
		sign, err = base64.RawURLEncoding.DecodeString(encodedSign)
		if err != nil {
			err = fmt.Errorf("sign format error: %w", err)
			return
		}

		body := []byte(encodedHeader + "." + encodedPayload)
		Domain := header["D"]

		var newVerifyKey []byte
		if key, ok := header["K"]; ok {
			newVerifyKey, err = base64.RawURLEncoding.DecodeString(key)
			if err != nil {
				err = fmt.Errorf("verify key format error: %w", err)
				return
			}
		}

		var verifyKey []byte
		verifyKey, err = n.GetVerifyKey(Domain, Algorithm)
		if err != nil {
			return
		}

		if verifyKey == nil {
			if newVerifyKey == nil {
				err = ErrVerifyKeyNotFound
				return
			} else {
				verifyKey = newVerifyKey
			}
		} else if bytes.Equal(newVerifyKey, verifyKey) {
			newVerifyKey = nil
		}

		if !ed25519.Verify(verifyKey, body, sign) {
			err = ErrVerifySign
			return
		}

		if newVerifyKey != nil {
			err = n.SetVerifyKey(Domain, Algorithm, newVerifyKey)
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
