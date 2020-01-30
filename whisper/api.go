package whisper

import (
	"fmt"
	ecies "github.com/ecies/go"
)

// NewMessage contain all the fields to create a whisper message
type NewMessage struct {
	SymKeyID  string
	PublicKey []byte
	TTL       uint32
	Topic     Topic
	PowTime   uint32
	Payload   []byte
}

// NewWhisperMessage create new whisper message and send it to its peers
func (whisper *Whisper) NewWhisperMessage(message NewMessage) ([]byte, error) {

	isSymKey := len(message.SymKeyID) > 0
	isPubKey := len(message.PublicKey) > 0

	// either symmetric or asymmetric key
	if isSymKey && isPubKey {
		return nil, fmt.Errorf("specifiy either public or symmetric key")
	}

	params := &MessageParams{
		TTL:     message.TTL,
		Payload: message.Payload,
		PowTime: message.PowTime,
		Topic:   message.Topic,
	}

	// check crypt keys
	if isSymKey {
		if params.Topic == (Topic{}) {
			return nil, fmt.Errorf("need topic with symmetric key")
		}
		key, err := whisper.GetSymKeyFromID(message.SymKeyID)
		if err != nil {
			return nil, err
		}
		params.KeySym = key

		if len(params.KeySym) != aesKeyLength {
			return nil, fmt.Errorf("invalid key length")
		}
	}

	if isPubKey {
		key, err := ecies.NewPublicKeyFromBytes(message.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("invalid public key")
		}
		params.Dst = key
	}

	// encrypt and create envelope
	var result []byte
	env, err := params.GetEnvelopeFromMessage()
	if err != nil {
		return nil, err
	}

	err = whisper.SendEnvelope(env)
	if err == nil {
		fmt.Println("[WHISPER] sent whisper envelope")
		hash := env.GetHash()
		result = hash[:]
	}

	return result, err
}

// FilterOptions holds various filter options for messages
type FilterOptions struct {
	SymKeyID     string
	PrivateKeyID string
	MinPow       float64
	Topics       []Topic
}

// GetFilterMessages returns the messages that match the filter criteria
func (whisper *Whisper) GetFilterMessages(id string) ([]*ReceivedMessage, error) {
	f := whisper.filters.GetFilterFromID(id)
	if f == nil {
		return nil, fmt.Errorf("filter not found")
	}

	receivedMessages := f.GetReceivedMessagesFromFilter()
	messages := make([]*ReceivedMessage, 0, len(receivedMessages))
	for _, msg := range receivedMessages {
		messages = append(messages, msg)
	}

	return messages, nil
}

// NewMessageFilter creates a new filter
func (whisper *Whisper) NewMessageFilter(req FilterOptions) (string, error) {

	filter := &Filter{}

	isSymKey := len(req.SymKeyID) > 0
	isPrivKey := len(req.PrivateKeyID) > 0

	// either symmetric or asymmetric key
	if isSymKey && isPrivKey {
		return "", fmt.Errorf("either private or symmetric key")
	}

	if isSymKey {
		key, err := whisper.GetSymKeyFromID(req.SymKeyID)
		if err != nil {
			return "", fmt.Errorf("no symmetric key found")
		}
		filter.KeySym = key
		if len(key) != aesKeyLength {
			return "", fmt.Errorf("invalid key length")
		}
	}

	if isPrivKey {
		key, err := whisper.GetPrivateKey(req.PrivateKeyID)
		if err != nil {
			return "", fmt.Errorf("no private key")
		}
		filter.KeyAsym = key
	}

	topics := make([][]byte, len(req.Topics))
	if len(req.Topics) > 0 {
		for i, topic := range req.Topics {
			topics[i] = make([]byte, TopicLength)
			copy(topics[i], topic[:])
		}
	}

	f := &Filter{
		KeySym:   filter.KeySym,
		KeyAsym:  filter.KeyAsym,
		Topics:   topics,
		Pow: 	req.MinPow,
		Messages: make(map[[32]byte]*ReceivedMessage),
	}

	s, err := whisper.filters.AddFilter(f)
	if err == nil {
		whisper.updateBloomFilter(f)
	}

	fmt.Println("[WHISPER] created new filter with id = " + s)

	return s, err
}
