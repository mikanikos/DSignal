// Contributors: Sergio Roldan

package adssignal

import (
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

// GenerateDH Returns d,Q with d a 256 bits integer represented as a byte array and Q and elliptic curve point (dG with G the generator of the curve) with point compression
// represented as a byte array
func GenerateDH() *DHPair {
	p256 := elliptic.P256()
	d, Qx, Qy, err := elliptic.GenerateKey(p256, rand.Reader)

	if err != nil {
		fmt.Println(err)
	}
	if !p256.IsOnCurve(Qx, Qy) {
		fmt.Println(err)
	}

	pubKey := EllipticPoint{
		p256,
		Qx,
		Qy,
	}

	dhPair := DHPair{
		&pubKey,
		&PrivateKey{
			&d,
		},
	}

	return &dhPair
}

// GenerateAKA a 20 bytes quasi-identifier of a Peer 
func GenerateAKA(pubKey EllipticPoint) []byte {
	marshalPk := Marshal(pubKey.x, pubKey.y)
	hash := sha256.Sum256(marshalPk)

	return hash[12:32]
}

// DH computation in a sense of res = integer * elliptic point
func DH(dhPair DHPair, dhPubKey EllipticPoint) []byte {
	Kx, Ky := (dhPubKey.C).ScalarMult(dhPubKey.x, dhPubKey.y, *dhPair.PrivKey.d)

	K := Marshal(Kx, Ky)
	return K
}
