package notary

import (
	"errors"
	"sync"
)

type LocalState sync.Map

type slot struct {
	Algorithm Alg
	Key       []byte
}

func (s *LocalState) GetKey(Domain string) (Algorithm Alg, Key []byte, err error) {
	m := (*sync.Map)(s)
	v, ok := m.Load(Domain)
	if !ok {
		return
	}

	k, ok := v.(*slot)
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
	m.Store(Domain, &slot{Algorithm, Key})
	return
}
