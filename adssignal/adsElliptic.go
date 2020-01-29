package adssignal

import (
	"crypto/elliptic"
	"math/big"
)

// Defines the parameter for curve calculations
var bigTHREE = big.NewInt(3)

// The elliptic point representation
type EllipticPoint struct {
	C    elliptic.Curve
	x, y *big.Int
}

// Private key representation
type PrivateKey struct {
	d *[]byte
}

// A DH pair including private and public key
type DHPair struct {
	PubKey  *EllipticPoint
	PrivKey *PrivateKey
}

// Compress a given point
func CompressPoint(Q EllipticPoint) []byte {
	return Marshal(Q.x, Q.y)
}

// Uncompress a given compressed point
func UncompressPoint(data []byte) EllipticPoint {
	x, y, C := Unmarshal(data)

	return EllipticPoint{
		C, x, y,
	}
}

// Marshal an elliptic point x,y to its compressed representation
func Marshal(x, y *big.Int) []byte {
	curve := elliptic.P256()
	byteLen := (curve.Params().BitSize + 7) >> 3

	ret := make([]byte, 1+byteLen)
	ret[0] = 2 + byte(y.Bit(0)) // compressed point: 2 if y is even or 3 if it is odd

	xBytes := x.Bytes()
	copy(ret[1+byteLen-len(xBytes):], xBytes)
	return ret
}

// Unmarshal a marshal elliptic point (compressed or uncompressed)
func Unmarshal(data []byte) (x, y *big.Int, C elliptic.Curve) {
	curve := elliptic.P256()
	byteLen := (curve.Params().BitSize + 7) >> 3

	// uncompressed point fallback to elliptic marshal
	if len(data) != 1+byteLen || data[0] != 0x02 && data[0] != 0x03 {
		x, y = elliptic.Unmarshal(curve, data)
		return x, y, curve
	}

	// Obtain x
	x = new(big.Int).SetBytes(data[1 : 1+byteLen])
	y = new(big.Int)

	// y' = x^3 - 3x + b -> short Weierstrass form with a = -3 as stated in https://golang.org/pkg/crypto/elliptic/ and https://www.hyperelliptic.org/EFD/g1p/auto-shortw.html
	y.Mul(x, x)
	y.Sub(y, bigTHREE)
	y.Mul(y, x)
	y.Add(y, curve.Params().B)
	y.Mod(y, curve.Params().P)

	// Obtain y computing the square root of y' = y²
	ModSqrt(y, curve, y)

	// If y's parity is different to data's parity substract P to obtain a matching parity between y and the compressed point marshal
	if y.Bit(0) != uint(data[0]&0x01) {
		y.Sub(curve.Params().P, y)
	}

	return x, y, curve
}

// Compare two given points
func ComparePoints(p1 EllipticPoint, p2 EllipticPoint) bool {
	if p1.x.Cmp(p2.x) != 0 || p1.y.Cmp(p2.y) != 0 {
		return false
	}

	return true
}

// Compute the square root a = sqrt(a) (mod curve_p) in the curve
func ModSqrt(z *big.Int, curve elliptic.Curve, a *big.Int) *big.Int {
	p1 := big.NewInt(1)
	p1.Add(p1, curve.Params().P)

	result := big.NewInt(1)

	for i := p1.BitLen() - 1; i > 1; i-- {
		result.Mul(result, result)
		result.Mod(result, curve.Params().P)
		if p1.Bit(i) > 0 {
			result.Mul(result, a)
			result.Mod(result, curve.Params().P)
		}
	}

	z.Set(result)
	return z
}
