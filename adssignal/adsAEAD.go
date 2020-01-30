// Contributors: Sergio Roldan

package adssignal

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"encoding/gob"
	"log"
)

// AESBlock Size of AES block in bytes
const AESBlock = 16

// EncryptAEAD the plaintext plaintext including the aditional data ad using the key mk
func EncryptAEAD(mk []byte, plaintext string, ad []byte) RatchetCipher {
	// Derivate the key
	encryptionKey, authenticationKey, IV := KdfAEAD(mk)

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		panic(err.Error())
	}

	// Encrypt the plain text
	plainbytes := []byte(plaintext)
	paddedPt := padd(plainbytes)

	S := make([]byte, len(paddedPt))

	mode := cipher.NewCBCEncrypter(block, IV)
	mode.CryptBlocks(S, paddedPt)

	// Parse the additional data
	adLen := len(ad)
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, uint64(adLen))
	adS := append(ad, S...)
	adSadLen := append(adS, bs[:]...)

	// Get the tag and append to cipher
	tag := NewHMAC(adSadLen, authenticationKey)
	chiper := append(S, tag[:AESBlock]...)

	// Return both
	ratchetCipher := RatchetCipher{
		ad,
		chiper,
	}

	return ratchetCipher
}

// DecryptAEAD the ciphertext cipher including the header header using the key mk
func DecryptAEAD(mk []byte, ciph RatchetCipher, header RatchetHeader) *string {
	// Derivate the key
	encryptionKey, authenticationKey, IV := KdfAEAD(mk)

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		panic(err.Error())
	}

	// Get cipher and tag
	ad := Concat(ciph.Ad, header)
	ciphertext := ciph.Ciph[:len(ciph.Ciph)-AESBlock]
	expectedTag := ciph.Ciph[len(ciph.Ciph)-AESBlock:]

	// Parse the additional data
	adLen := len(ad)
	bs := make([]byte, 8)
	binary.LittleEndian.PutUint64(bs, uint64(adLen))
	adS := append(ad, ciphertext...)
	adSadLen := append(adS, bs[:]...)

	// Compare the tags
	cipherTag := NewHMAC(adSadLen, authenticationKey)
	comp := bytes.Compare(expectedTag, cipherTag[:16])

	if comp != 0 {
		return nil
	}

	// Decrypt the cipher
	S := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, IV)
	mode.CryptBlocks(S, ciphertext)

	plainbyte := unpadd(S)
	plaintext := string(plainbyte)
	return &plaintext
}

// Padd plaintext to AES block length
func padd(plaintext []byte) []byte {
	paddedPt := plaintext
	lastBlockLen := len(plaintext) % AESBlock

	paddLen := AESBlock - lastBlockLen

	if paddLen == 0 {
		paddLen = AESBlock
	}

	bs := make([]byte, 2)
	binary.LittleEndian.PutUint16(bs, uint16(paddLen))

	for i := 0; i < paddLen; i++ {
		paddedPt = append(paddedPt, bs[:1]...)
	}

	return paddedPt
}

// Unpadd plaintext from AES block length
func unpadd(plaintext []byte) []byte {
	possPadd := int(plaintext[len(plaintext)-1])

	if possPadd > 0 && possPadd <= 16 {
		for i := 1; i < possPadd; i++ {
			if int(plaintext[len(plaintext)-1-i]) != possPadd {
				return plaintext
			}
		}

		return plaintext[:len(plaintext)-possPadd]
	}

	return plaintext
}

// Concat the header to the additional data
func Concat(ad []byte, header RatchetHeader) []byte {
	var network bytes.Buffer
	enc := gob.NewEncoder(&network)

	err := enc.Encode(header)
	if err != nil {
		log.Fatal("encode error:", err)
	}

	return network.Bytes()
}