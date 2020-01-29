package whisper

import (
	"crypto/aes"
	"crypto/cipher"
	crand "crypto/rand"
	"encoding/binary"
	"github.com/dedis/protobuf"
	ecies "github.com/ecies/go"
	"golang.org/x/crypto/sha3"
	"io"
	"math/big"
	"time"
)

// MessageParams specifies all the parameters needed to prepare an envelope
type MessageParams struct {
	Src     *ecies.PrivateKey
	Dst     *ecies.PublicKey
	KeySym  []byte
	Topic   Topic
	PowTime uint32
	TTL     uint32
	Payload []byte
	Padding []byte
}

//ReceivedMessage is a message received and decrypted
type ReceivedMessage struct {
	Sent    uint32
	TTL     uint32
	Src     *ecies.PublicKey
	Dst     *ecies.PublicKey
	Payload []byte
	Topic   Topic

	SymKeyHash   [32]byte
	EnvelopeHash [32]byte
}

// appendPadding appends random padding to the message
func (params *MessageParams) appendPadding() error {
	bytes, err := protobuf.Encode(params)
	if err != nil {
		return err
	}
	rawSize := len(bytes)
	odd := rawSize % padSizeLimit
	paddingSize := padSizeLimit - odd

	pad := make([]byte, paddingSize)
	_, err = crand.Read(pad)
	if err != nil {
		return err
	}
	params.Payload = pad
	return nil
}

// encryptWithPublicKey encrypts a message with a public key.
func encryptWithPublicKey(data []byte, key *ecies.PublicKey) ([]byte, error) {
	return ecies.Encrypt(key, data)
}

// use aes-gcm
func encryptWithSymmetricKey(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(crand.Reader, nonce)
	if err != nil {
		return nil, err
	}
	encrypted := gcm.Seal(nonce, nonce, data, nil)
	return encrypted, nil
}

// GetEnvelopeFromMessage encrypts the message and prepare the Envelope
func (params *MessageParams) GetEnvelopeFromMessage() (envelope *Envelope, err error) {
	if params.TTL == 0 {
		params.TTL = DefaultTTL
	}

	var encrypted []byte

	if params.Dst != nil {
		encrypted, err = encryptWithPublicKey(params.Payload, params.Dst)
		if err != nil {
			return nil, err
		}
	} else if params.KeySym != nil {
		encrypted, err = encryptWithSymmetricKey(params.Payload, params.KeySym)
		if err != nil {
			return nil, err
		}
	} else {
		encrypted = params.Payload
	}
	//if err != nil {
	//	return nil, err
	//}

	envelope = NewEnvelope(params.TTL, params.Topic, encrypted)

	// add nonce for requiring enough pow
	envelope.addNonceForPow(params)

	return envelope, nil
}

func (e *Envelope) addNonceForPow(options *MessageParams) {

	e.Expiry += options.PowTime

	bestLeadingZeros := 0

	encodedEnvWithoutNonce, _ := protobuf.Encode([]interface{}{e.Expiry, e.TTL, e.Topic, e.Data})
	buf := make([]byte, len(encodedEnvWithoutNonce)+8)
	copy(buf, encodedEnvWithoutNonce)

	finish := time.Now().Add(time.Duration(options.PowTime) * time.Second).UnixNano()
	for nonce := uint64(0); time.Now().UnixNano() < finish; {
		for i := 0; i < 1024; i++ {
			binary.BigEndian.PutUint64(buf[len(encodedEnvWithoutNonce):], nonce)
			d := sha3.New256()
			d.Write(buf)
			powHash := new(big.Int).SetBytes(d.Sum(nil))
			leadingZeros := 256 - powHash.BitLen()
			if leadingZeros > bestLeadingZeros {
				e.Nonce, bestLeadingZeros = nonce, leadingZeros
			}
			nonce++
		}
	}

	e.computePow(0)
}

// decryptWithSymmetricKey decrypts a message with symmetric key
func decryptWithSymmetricKey(encrypted []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	nonce, ciphertext := encrypted[:nonceSize], encrypted[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
}

// decryptWithPrivateKey decrypts an encrypted payload with private key
func decryptWithPrivateKey(encrypted []byte, key *ecies.PrivateKey) ([]byte, error) {
	return ecies.Decrypt(key, encrypted)
}
