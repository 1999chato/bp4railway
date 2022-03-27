package main

import (
	"bypaths/notary"
	"encoding/base64"
	"flag"
	"fmt"
)

/*
按参数生成 token
*/
func main() {
	Domain := flag.String("domain", "", "domain")
	Algorithm := flag.String("alg", "ed25519", "alg")
	signkey := flag.String("signkey", "", "signkey")
	VerifyKey := flag.String("verifykey", "", "verifykey")
	flag.Parse()
	args := flag.Args()
	// fmt.Println("args:", args)
	if len(args) < 1 || *Domain == "" || *signkey == "" {
		flag.Usage()
		return
	}
	Payload := args[0]
	SignKey, err := base64.RawURLEncoding.DecodeString(*signkey)
	if err != nil {
		panic(err)
	}

	notarize := notary.Notarize{}

	token, err := notarize.EncodeToken([]byte(Payload), *Domain, notary.Alg(*Algorithm), SignKey, []byte(*VerifyKey))
	if err != nil {
		panic(err)
	}
	fmt.Println("token:", token)
}
