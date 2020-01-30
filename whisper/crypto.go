/* Contributors: Andrea Piccione */

package whisper

import (
	"encoding/hex"
	"fmt"
	ecies "github.com/ecies/go"
)

// NewKeyPair generates secp256k1 key pair and store them
func (whisper *Whisper) NewKeyPair() (string, error) {
	key, err := ecies.GenerateKey()
	id, err := generateRandomID()
	if err != nil {
		return "", err
	}

	fmt.Println(hex.EncodeToString(key.Bytes()))

	if _, loaded := whisper.cryptoKeys.LoadOrStore(id, key); loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	return id, nil
}

// DeleteKey deletes the specified key with an id
func (whisper *Whisper) DeleteKey(id string) {
	whisper.cryptoKeys.Delete(id)
}

// AddKeyPair imports an asymmetric private key and returns its id
func (whisper *Whisper) AddKeyPair(key *ecies.PrivateKey) (string, error) {
	id, err := generateRandomID()
	if err != nil {
		return "", err
	}

	if _, loaded := whisper.cryptoKeys.LoadOrStore(id, key); loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}

	return id, nil
}

// HasKey checks if a key is present in the storage
func (whisper *Whisper) HasKey(id string) bool {
	_, loaded := whisper.cryptoKeys.Load(id)
	return loaded
}

// GetPrivateKey returns the private key given an id
func (whisper *Whisper) GetPrivateKey(id string) (*ecies.PrivateKey, error) {
	value, loaded := whisper.cryptoKeys.Load(id)
	if loaded {
		return value.(*ecies.PrivateKey), nil
	}
	return nil, fmt.Errorf("no private key for id given")
}

// GenerateSymKey generates a random symmetric key and stores it under id
func (whisper *Whisper) GenerateSymKey() (string, error) {
	key, err := generateRandomBytes(aesKeyLength)
	if err != nil {
		return "", err
	}

	id, err := generateRandomID()
	if err != nil {
		return "", err
	}


	fmt.Println(hex.EncodeToString(key))
	if _, loaded := whisper.cryptoKeys.LoadOrStore(id, key); loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	return id, nil
}

// AddSymKey stores a symmetric key returns its id
func (whisper *Whisper) AddSymKey(k string) (string, error) {
	key, _ := hex.DecodeString(k)
	if len(key) != aesKeyLength {
		return "", fmt.Errorf("wrong key GetSize")
	}

	id, err := generateRandomID()
	if err != nil {
		return "", err
	}

	if _, loaded := whisper.cryptoKeys.LoadOrStore(id, key); loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}
	return id, nil
}

// GetSymKeyFromID returns the symmetric key given its id
func (whisper *Whisper) GetSymKeyFromID(id string) ([]byte, error) {
	value, loaded := whisper.cryptoKeys.Load(id)
	if loaded {
		return value.([]byte), nil
	}
	return nil, fmt.Errorf("no sym key for id given")
}

// AddPrivateKey adds the given private key
func (whisper *Whisper) AddPrivateKey(privateKey []byte) (string, error) {
	key := ecies.NewPrivateKeyFromBytes(privateKey)
	return whisper.AddKeyPair(key)
}

// GetPublicKeyFromID returns the public key given an id
func (whisper *Whisper) GetPublicKeyFromID(id string) ([]byte, error) {
	key, err := whisper.GetPrivateKey(id)
	if err != nil {
		return []byte{}, err
	}
	return key.PublicKey.Bytes(false), nil
}

// GetPrivateKeyFromID returns the private key given an id
func (whisper *Whisper) GetPrivateKeyFromID(id string) ([]byte, error) {
	key, err := whisper.GetPrivateKey(id)
	if err != nil {
		return []byte{}, err
	}
	return key.Bytes(), nil
}
