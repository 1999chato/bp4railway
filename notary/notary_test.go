package notary

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeToken(t *testing.T) {
	notarize := Notarize{
		State: &LocalState{},
	}
	verifykey, signkey, err := notarize.GenerateKey(Ed25519Alg)
	assert.NoError(t, err)
	body := []byte("hello world")
	domain := "test"
	token, err := notarize.EncodeToken(body, domain, Ed25519Alg, signkey, verifykey)
	assert.NoError(t, err)
	t.Logf("token:%v\n", token)
	normaltoken, err := notarize.EncodeToken(body, domain, Ed25519Alg, signkey, nil)
	assert.NoError(t, err)
	t.Logf("token:%v\n", normaltoken)
}

func TestDecodeToken(t *testing.T) {
	notarize := Notarize{
		State: &LocalState{},
	}

	initToken := "eyJBIjoiZWQyNTUxOSIsIkQiOiJ0ZXN0IiwiSyI6ImktZXo0UnZGTUJqbWVHQlVISVRlVzhuTEhIaHJNeF9mN3Bnb1lRSjRhenMifQ.aGVsbG8gd29ybGQ.xra8DjQ5KAnQ1sSSU3na7TsEVU22IYQGuBaitiFXM5XPZ1YdjbmMIgUSdIYvHXuM_gW9ThMgVMQhtdIVletOAw"
	token := "eyJBIjoiZWQyNTUxOSIsIkQiOiJ0ZXN0In0.aGVsbG8gd29ybGQ.rqjyZC8sw37vntX1JERXYaGyChoBKVJIFeL6-5kaFMrBL_kcx6Uw2Zq-94V1hXqJiGi6REc1un9umr5B7jl2Aw"

	payload, _, _, _, _, _, err := notarize.DecodeToken(token)
	assert.Error(t, err)
	payload, _, _, _, _, _, err = notarize.DecodeToken(initToken)
	assert.NoError(t, err)
	t.Logf("payload:%s\n", payload)
	payload, _, _, _, _, _, err = notarize.DecodeToken(token)
	assert.NoError(t, err)
	t.Logf("payload:%s\n", payload)
}
