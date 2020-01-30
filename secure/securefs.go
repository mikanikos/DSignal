package secure

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/dedis/protobuf"
)

// SignedFile ...
type SignedFile struct {
	Data      []byte
	Signature []byte
}

// EncryptedFile ...
type EncryptedFile struct {
	PlainTextLength int    // length of the plain text
	EncryptedData   []byte // encrypted bytes of serialized SignedFile
	EncryptedKey    []byte // key used to encrypt data encrypcted in turn wtih recepients public key
}

// SignData signs a byte array using a private key.
func SignData(data []byte, signKey *rsa.PrivateKey) (*SignedFile, error) {
	rng := rand.Reader
	hashed := sha256.Sum256(data)
	signature, err := rsa.SignPKCS1v15(rng, signKey, crypto.SHA256, hashed[:])
	return &SignedFile{Data: data, Signature: signature}, err
}

// VerifySignedData verifies a signed file using a public key.
func VerifySignedData(signedFile *SignedFile, signKey *rsa.PublicKey) bool {
	hashed := sha256.Sum256(signedFile.Data)
	err := rsa.VerifyPKCS1v15(signKey, crypto.SHA256, hashed[:], signedFile.Signature)
	return err == nil
}

// EncryptFile encrypts a signed file using a random key and encrypts the key using a public key.
func EncryptFile(signedFile *SignedFile, encryptPublicKey *rsa.PublicKey) *EncryptedFile {
	// generate a random key
	key := make([]byte, 32)
	rand.Read(key)

	plaintext, _ := protobuf.Encode(signedFile)
	plainTextLength := len(plaintext)

	// pad bytes
	paddingLen := aes.BlockSize - plainTextLength%aes.BlockSize
	padding := make([]byte, paddingLen)
	rand.Read(padding)
	plaintext = append(plaintext, padding...)

	block, _ := aes.NewCipher(key)

	// initialization vector
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	rand.Read(iv)

	// encrypt
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)

	// encrypt key
	encryptedKey, _ := rsa.EncryptOAEP(sha256.New(), rand.Reader, encryptPublicKey, key, make([]byte, 0))

	return &EncryptedFile{PlainTextLength: plainTextLength, EncryptedData: ciphertext, EncryptedKey: encryptedKey}
}

// DecryptFile decrypts an encrypted file using the private key of the intended receipient.
func DecryptFile(encryptedFile *EncryptedFile, privateKey *rsa.PrivateKey) *SignedFile {
	// decrypt key
	key, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, privateKey, encryptedFile.EncryptedKey, make([]byte, 0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decrypting key: %v\n", err)
		return nil
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating the cipher object: %v\n", err)
		return nil
	}

	if len(encryptedFile.EncryptedData) < aes.BlockSize {
		fmt.Fprintf(os.Stderr, "Ciphertext too short/\n")
		return nil
	}

	iv := encryptedFile.EncryptedData[:aes.BlockSize]
	ciphertext := encryptedFile.EncryptedData[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		fmt.Fprintf(os.Stderr, "Ciphertext is not a multiple of the block size.\n")
		return nil
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	signedFile := &SignedFile{}
	err = protobuf.Decode(ciphertext[:encryptedFile.PlainTextLength], signedFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error decrypting file.\n")
		return nil
	}

	return signedFile
}
