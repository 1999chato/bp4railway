package notary

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateToken(t *testing.T) {
	notarize := Notarize{}
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
