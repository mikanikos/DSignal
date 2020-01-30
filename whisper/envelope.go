/* Contributors: Andrea Piccione */

package whisper

import (
	"github.com/dedis/protobuf"
	"golang.org/x/crypto/sha3"
	"time"
)

// Envelope according to Whisper protocol
type Envelope struct {
	Expiry uint32
	TTL    uint32
	Topic  Topic
	Data   []byte
	Nonce  uint64

	pow   float64
	bloom []byte
}

// NewEnvelope creates new envelope
func NewEnvelope(ttl uint32, topic Topic, data []byte) *Envelope {
	return &Envelope{
		Expiry: uint32(time.Now().Add(time.Duration(ttl) * time.Second).Unix()),
		TTL:    ttl,
		Topic:  topic,
		Data:   data,
		Nonce:  0,
	}
}

//GetSize returns the GetSize of envelope
func (e *Envelope) GetSize() int {
	encodedEnvelope, _ := protobuf.Encode(e)
	return len(encodedEnvelope)
}

// GetPow compute pow if not already done
func (e *Envelope) GetPow() float64 {
	if e.pow == 0 {
		e.computePow(0)
	}
	return e.pow
}

// GetBloom compute bloom if not already done
func (e *Envelope) GetBloom() []byte {
	if e.bloom == nil {
		e.bloom = ConvertTopicToBloom(e.Topic)
	}
	return e.bloom
}

// GetHash returns the SHA3 hash of the envelope, calculating it if not yet done.
func (e *Envelope) GetHash() [32]byte {
	encoded, _ := protobuf.Encode(e)
	return sha3.Sum256(encoded)
}

// GetMessageFromEnvelope decrypts the message payload of the envelope
func (e *Envelope) GetMessageFromEnvelope(subscriber *Filter) *ReceivedMessage {
	if subscriber == nil {
		return nil
	}

	msg := &ReceivedMessage{}

	if subscriber.KeyAsym != nil {
		decrypted, err := decryptWithPrivateKey(e.Data, subscriber.KeyAsym)
		msg.Payload = decrypted
		if err != nil {
			return nil
		}
		if decrypted != nil {
			msg.Dst = subscriber.KeyAsym.PublicKey
		}
	} else if subscriber.KeySym != nil {
		decrypted, err := decryptWithSymmetricKey(e.Data, subscriber.KeySym)
		msg.Payload = decrypted
		if err != nil {
			return nil
		}
		if decrypted != nil {
			msg.SymKeyHash = sha3.Sum256(subscriber.KeySym)
		}
	} else {
		msg.Payload = e.Data
	}

	msg.Topic = e.Topic
	msg.TTL = e.TTL
	msg.Sent = e.Expiry - e.TTL
	msg.EnvelopeHash = e.GetHash()

	return msg
}
