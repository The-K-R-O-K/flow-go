package main

import (
	"crypto/rand"
	"fmt"

	"github.com/onflow/crypto"
)

func main() {
	fmt.Println("main starts")
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	rand.Read(seed)
	sk, _ := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)

	pk := sk.PublicKey()
	pkBytes := pk.Encode()
	pkCheck, _ := crypto.DecodePublicKey(crypto.BLSBLS12381, pkBytes)

	if !pk.Equals(pkCheck) {
		fmt.Println("noooo")
		return
	}
	hasher := crypto.NewExpandMsgXOFKMAC128("test tag")
	s, _ := sk.Sign(seed, hasher)
	result, _ := pk.Verify(s, seed, hasher)

	if result {
		fmt.Println("works!!")
	} else {
		fmt.Println("nooo")
	}
}
