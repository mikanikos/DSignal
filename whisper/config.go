package whisper

import (
	"time"
)

// main whisper protocol parameters, from official source
const (
	statusCode         = 0
	messagesCode       = 1
	powRequirementCode = 2
	bloomFilterExCode  = 3

	// lengths in bytes
	TopicLength     = 4
	aesKeyLength    = 32
	keyIDSize       = 32
	BloomFilterSize = 64

	MaxMessageSize        = uint32(10 * 1024 * 1024) // maximum accepted GetSize of a message.
	DefaultMaxMessageSize = uint32(1024 * 1024)
	DefaultMinimumPoW     = 0.2
	DefaultTTL            = 60 // in seconds
	DefaultSyncAllowance  = 10 // seconds

	padSizeLimit      = 256
	messageQueueLimit = 1024

	expirationTimer = 4 * time.Second
	broadcastTimer  = 2 * time.Second
	statusTimer     = 2 * time.Second
)

const (
	maxMsgSizeIdx           = iota
	minPowIdx
	minPowToleranceIdx
	bloomFilterIdx
	bloomFilterToleranceIdx
)
