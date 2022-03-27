package main

import (
	"bypaths/notary"
	"encoding/base64"
	"fmt"
)

func main() {
	notarize := notary.Notarize{}

	verifykey, signkey, err := notarize.GenerateKey(notary.Ed25519Alg)
	if err != nil {
		panic(err)
	}

	verifykeyEncoded := base64.RawURLEncoding.EncodeToString(verifykey)
	signkeyEncoded := base64.RawURLEncoding.EncodeToString(signkey)
	fmt.Println("algorithm:", string(notary.Ed25519Alg))
	fmt.Println("verifykey:", verifykeyEncoded)
	fmt.Println("  signkey:", signkeyEncoded)
}
