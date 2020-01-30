// Authors: Sergio Roldan

package adssignal

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
)

// ECDSA Signature representation
type ECDSA struct {
	R []byte
	S []byte
}

// GenerateSignature a new signature of message using the private key
func GenerateSignature(message []byte, sk *PrivateKey, pk EllipticPoint) *ECDSA {
	privateKey := ConvertKey(sk, pk)
	hash := sha256.Sum256(message)

	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return &ECDSA{
		r.Bytes(),
		s.Bytes(),
	}
}

// VerifySignature the given signature given the message and the public key
func VerifySignature(message []byte, sign ECDSA, pk EllipticPoint) bool {
	privateKey := ConvertKey(nil, pk)
	hash := sha256.Sum256(message)

	return ecdsa.Verify(&privateKey.PublicKey, hash[:], new(big.Int).SetBytes(sign.R), new(big.Int).SetBytes(sign.S))
}

// ConvertKey a DH key pair in a edcsa valid key pair
func ConvertKey(sk *PrivateKey, pk EllipticPoint) *ecdsa.PrivateKey {
	pubKey := ecdsa.PublicKey{
		Curve: pk.C,
		X:     pk.x,
		Y:     pk.y,
	}

	var D *big.Int

	if sk != nil {
		D = new(big.Int)
		D.SetBytes(*sk.d)
	}

	privKey := ecdsa.PrivateKey{
		PublicKey: pubKey,
		D:         D,
	}

	return &privKey
}
