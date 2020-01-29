package whisper

import (
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/dedis/protobuf"
	"golang.org/x/crypto/sha3"
	gmath "math"
	"math/big"
)

// GenerateRandomID generates a random string
func GenerateRandomID() (id string, err error) {
	buf, err := generateRandomBytes(keyIDSize)
	if err != nil {
		return "", err
	}
	if len(buf) != keyIDSize {
		return "", fmt.Errorf("error in generateRandomID: crypto/rand failed to generate random data")
	}
	id = hex.EncodeToString(buf)
	return id, err
}

func generateRandomBytes(length int) ([]byte, error) {
	array := make([]byte, length)
	_, err := crand.Read(array)
	if err != nil {
		return nil, err
	}
	return array, nil
}

func (e *Envelope) computePow(diff uint32) {
	encodedEnvWithoutNonce, _ := protobuf.Encode([]interface{}{e.Expiry, e.TTL, e.Topic, e.Data})
	buf := make([]byte, len(encodedEnvWithoutNonce)+8)
	copy(buf, encodedEnvWithoutNonce)
	binary.BigEndian.PutUint64(buf[len(encodedEnvWithoutNonce):], e.Nonce)
	d := sha3.New256()
	d.Write(buf)
	powHash := new(big.Int).SetBytes(d.Sum(nil))
	leadingZeroes := 256 - powHash.BitLen()
	e.pow = gmath.Pow(2, float64(leadingZeroes)) / (float64(e.GetSize()) * float64(e.TTL+diff))
}