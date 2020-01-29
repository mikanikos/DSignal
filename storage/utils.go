package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"os"
	"reflect"
)

// MessageAddrPair represents a pair of a DStoreMessage and a destination to which it should be sent.
type MessageAddrPair struct {
	Message *DStoreMessage
	Address string
}

// Compare return -1, 0, or 1 if the integers represented by a and b satisfy a < b, a == b, or a > b respectively.
func Compare(a []byte, b []byte) int {
	if len(a) != len(b) {
		fmt.Fprintf(os.Stderr, "Compare Error: Lengths are not %d\n", DStoreIDSize)
		return 0
	}
	for i := len(a) - 1; i >= 0; i-- {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

// Min returns the minimum of two integers
func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// XorDistance computes the Xor distance between a and b.
func XorDistance(a []byte, b []byte) []byte {
	if len(a) != DStoreIDSize || len(b) != DStoreIDSize {
		fmt.Fprintf(os.Stderr, "XorDistance Error: Lengths are not %d\n", DStoreIDSize)
		return nil
	}
	c := make([]byte, DStoreIDSize)
	for i, x := range a {
		c[i] = x ^ b[i]
	}
	return c
}

// GetBit returns the i-th bit of a byte array starting from the least significant bit.
func GetBit(bytes []byte, index int) byte {
	return (bytes[index>>3] >> (uint(index) % 8)) & 1
}

// SetPrefix sets the most significant prefLen bits of bytes to those of pref.
func SetPrefix(bytes []byte, pref []byte, prefLen int) {
	for i := len(bytes) - 1; i >= 0; i-- {
		if prefLen >= 8 {
			bytes[i] = pref[i]
			prefLen -= 8
		} else {
			mask := byte(0)
			for j := 0; j < prefLen; j++ {
				mask |= (1 << (7 - uint(j)))
			}
			bytes[i] &= ^mask
			bytes[i] |= (mask & pref[i])
			return
		}
	}
}

// MaxPrefixLen finds the maximum length of the matching prefix.
func MaxPrefixLen(id []byte, target []byte) int {
	if len(target) != DStoreIDSize || len(id) != DStoreIDSize {
		fmt.Fprintf(os.Stderr, "MaxPrefix Error: Lengths are not %d\n", DStoreIDSize)
		return -1
	}
	res := 0
	for i := DStoreIDSize*8 - 1; i >= 0; i-- {
		if GetBit(id, i) != GetBit(target, i) {
			break
		}
		res++
	}
	return res
}

// GenerateRandomRPCID ...
func GenerateRandomRPCID(numBytes int) string {
	res := make([]byte, numBytes)
	rand.Read(res)
	return hex.EncodeToString(res)
}

// GetHash computes the hash for a given data according to a given hash type.
func GetHash(data []byte, hashType byte) []byte {
	switch hashType {
	case HashTypeSha224:
		temp := sha256.Sum224(data)
		return temp[:]
	case HashTypeSha256:
		temp := sha256.Sum256(data)
		return temp[:]
	case HashTypeSha384:
		temp := sha512.Sum384(data)
		return temp[:]
	case HashTypeSha512:
		temp := sha512.Sum512(data)
		return temp[:]
	case HashTypeSha512_224:
		temp := sha512.Sum512_224(data)
		return temp[:]
	case HashTypeSha512_256:
		temp := sha512.Sum512_256(data)
		return temp[:]
	}
	return make([]byte, 0)
}

// CheckHash checks the hash for a given data according to a given hash type.
func CheckHash(data []byte, expected []byte, hashType byte) bool {
	if len(expected) != HashLength[hashType] {
		fmt.Fprintf(os.Stderr, "CheckHash Error: lengths do not match!")
		return false
	}
	return reflect.DeepEqual(expected, GetHash(data, hashType))
}

// GetDStoreIDForName converts a common name of a node to a DStoreID which is a random-looking 224 bits encoded as a byte array.
func GetDStoreIDForName(name string) []byte {
	hash := sha256.Sum256([]byte(name))
	return hash[0:DStoreIDSize]
}

// str converts  a given byte slice to a hex string.
func str(id []byte) string {
	return hex.EncodeToString(id)
}
