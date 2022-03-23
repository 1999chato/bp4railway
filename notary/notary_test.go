package notary

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateToken(t *testing.T) {
	notarize := Notarize{
		State: &LocalState{},
	}
	verifykey, signkey, err := notarize.GenerateKey(Ed25519Alg)
	assert.NoError(t, err)
	body := []byte("hello world")
	domain := "test"
	token, err := notarize.GenerateToken(body, domain, Ed25519Alg, signkey, verifykey)
	assert.NoError(t, err)
	t.Logf("token:%v\n", token)
	normaltoken, err := notarize.GenerateToken(body, domain, Ed25519Alg, signkey, nil)
	assert.NoError(t, err)
	t.Logf("token:%v\n", normaltoken)
}

func TestGetPayloadFromToken(t *testing.T) {
	notarize := Notarize{
		State: &LocalState{},
	}

	initToken := "eyJBIjoiZWQyNTUxOSIsIkQiOiJ0ZXN0IiwiSyI6ImktZXo0UnZGTUJqbWVHQlVISVRlVzhuTEhIaHJNeF9mN3Bnb1lRSjRhenMifQ.aGVsbG8gd29ybGQ.xra8DjQ5KAnQ1sSSU3na7TsEVU22IYQGuBaitiFXM5XPZ1YdjbmMIgUSdIYvHXuM_gW9ThMgVMQhtdIVletOAw"
	token := "eyJBIjoiZWQyNTUxOSIsIkQiOiJ0ZXN0In0.aGVsbG8gd29ybGQ.rqjyZC8sw37vntX1JERXYaGyChoBKVJIFeL6-5kaFMrBL_kcx6Uw2Zq-94V1hXqJiGi6REc1un9umr5B7jl2Aw"

	payload, err := notarize.GetPayloadFromToken(token)
	assert.Error(t, err)
	payload, err = notarize.GetPayloadFromToken(initToken)
	assert.NoError(t, err)
	t.Logf("payload:%s\n", payload)
	payload, err = notarize.GetPayloadFromToken(token)
	assert.NoError(t, err)
	t.Logf("payload:%s\n", payload)
}
