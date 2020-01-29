package adssignal

import (
	"crypto/hmac"
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

// KDF constants
const InfoKDF = "ADS_DSE_2019_KDF"
const InfoAEAD = "SDA_2019_AES_DSE"
const InfoX3DH = "DSA_XDH_DSE_2019"
const MsgKeyInput = "1"
const ChainKeyInput = "2"

// Derivate a new pair of keys from the root key and the ouput of DH
func KdfRK(rk []byte, dhOut []byte) (*[]byte, *[]byte) {
	sha256 := sha256.New

	// Generate two 256-bit derived keys.
	hkdf := hkdf.New(sha256, rk, dhOut, []byte(InfoKDF))

	rootKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdf, rootKey); err != nil {
		panic(err)
	}
	chainKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdf, chainKey); err != nil {
		panic(err)
	}

	return &rootKey, &chainKey
}

// Derivate a new pair of keys from a given sending/receiving key
func KdfCK(ck []byte) (*[]byte, *[]byte) {
	chainKey := NewHMAC([]byte(ChainKeyInput), ck)
	messageKey := NewHMAC([]byte(MsgKeyInput), ck)

	return &chainKey, &messageKey
}

// Derivate a new key from input and key
func NewHMAC(input, key []byte) []byte {
	sha256 := sha256.New

	mac := hmac.New(sha256, key)
	mac.Write(input)
	return mac.Sum(nil)
}

// Derivate new keys for AEAD given a message key
func KdfAEAD(mk []byte) ([]byte, []byte, []byte) {
	sha256 := sha256.New

	var salt [80]byte
	hkdf := hkdf.New(sha256, mk, salt[:], []byte(InfoAEAD))

	encryptionKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdf, encryptionKey); err != nil {
		panic(err)
	}
	authenticationKey := make([]byte, 32)
	if _, err := io.ReadFull(hkdf, authenticationKey); err != nil {
		panic(err)
	}
	IV := make([]byte, 16)
	if _, err := io.ReadFull(hkdf, IV); err != nil {
		panic(err)
	}

	return encryptionKey, authenticationKey, IV
}

// Derivate a new key for X3DH
func KdfX3DH(km []byte) []byte {
	sha256 := sha256.New

	var salt [32]byte
	hkdf := hkdf.New(sha256, km, salt[:], []byte(InfoX3DH))

	SK := make([]byte, 32)
	if _, err := io.ReadFull(hkdf, SK); err != nil {
		panic(err)
	}

	return SK
}
