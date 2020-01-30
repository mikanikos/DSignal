/* Contributors: Andrea Piccione */

package whisper

import (
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/mikanikos/DSignal/gossiper"
	"net"
	"runtime"
	"sync"
	"time"
)

// SafeEnvelopes stores envelopes and handle them concurrently safe
type SafeEnvelopes struct {
	Envelopes map[[32]byte]*EnvelopeOrigin
	Mutex     sync.RWMutex
}

// SafeEnvelopes stores envelopes and handle them concurrently safe
type EnvelopeOrigin struct {
	Envelope *Envelope
	Origin   *net.UDPAddr
}

// Whisper is the main part of the whisper protocol
type Whisper struct {
	// gossiper as underlying protocol
	gossiper *gossiper.Gossiper
	// main parameters (pow and bloom filter)
	parameters sync.Map
	// filters added for envelopes
	filters *FilterStorage
	// crypto keys (both private and symmetric) storage with unique id
	cryptoKeys sync.Map
	// envelopes which are not expired yet
	envelopes *SafeEnvelopes
	// routing envelopes according to received bloom filters
	routingHandler *RoutingHandler
	// blacklist of peers
	blacklist map[string]struct{}

	messageQueue chan *Envelope
	quit         chan struct{} // channel used for graceful exit
}

// NewWhisper creates new Whisper instance
func NewWhisper(g *gossiper.Gossiper) *Whisper {

	whisper := &Whisper{
		gossiper:       g,
		parameters:     sync.Map{},
		cryptoKeys:     sync.Map{},
		envelopes:      &SafeEnvelopes{Envelopes: make(map[[32]byte]*EnvelopeOrigin)},
		filters:        NewFilterStorage(),
		routingHandler: NewRoutingHandler(),
		messageQueue:   make(chan *Envelope, messageQueueLimit),
		quit:           make(chan struct{}),
		blacklist:      make(map[string]struct{}),
	}

	whisper.parameters.Store(minPowIdx, DefaultMinimumPoW)
	whisper.parameters.Store(maxMsgSizeIdx, DefaultMaxMessageSize)
	whisper.parameters.Store(bloomFilterIdx, GetEmptyBloomFilter())

	return whisper
}

// SendEnvelope send the envelope to the peers
func (whisper *Whisper) SendEnvelope(envelope *Envelope) error {
	err := whisper.handleEnvelope(&EnvelopeOrigin{Envelope: envelope, Origin: whisper.gossiper.ConnectionHandler.GossiperData.Address})
	if err != nil {
		fmt.Println(err)
		return fmt.Errorf("failed to handle Envelope envelope, %s", err)
	}
	return err
}

// Run the whisper protocol
func (whisper *Whisper) Run() {
	go whisper.processWhisperPacket()
	go whisper.processWhisperStatus()
	go whisper.updateEnvelopes()
	go whisper.sendStatusPeriodically()

	numCPU := runtime.NumCPU()
	for i := 0; i < numCPU; i++ {
		go whisper.processQueue()
	}
}

// Stop protocol with the quit channel
func (whisper *Whisper) Stop() error {
	close(whisper.quit)
	fmt.Println("whisper stopped")
	return nil
}

// process incoming whisper status
func (whisper *Whisper) processWhisperStatus() {
	for extPacket := range gossiper.PacketChannels["whisperStatus"] {
		//fmt.Println("\nWhisper: received status packet from peer " + extPacket.SenderAddr.String())
		if _, loaded := whisper.blacklist[extPacket.SenderAddr.String()]; !loaded {
			whisper.routingHandler.updateRoutingTable(extPacket.Packet.WhisperStatus, extPacket.SenderAddr)
			whisper.gossiper.HandleGossipMessage(extPacket, extPacket.Packet.WhisperStatus.Origin, extPacket.Packet.WhisperStatus.ID)
		}
	}
}

// process incoming whisper envelopes
func (whisper *Whisper) processWhisperPacket() {
	for extPacket := range gossiper.PacketChannels["whisperPacket"] {
		//fmt.Println("\nWhisper: received envelope from peer " + extPacket.SenderAddr.String())
		if _, loaded := whisper.blacklist[extPacket.SenderAddr.String()]; !loaded {

			packet := extPacket.Packet.WhisperPacket

			if packet.Code == messagesCode {
				// decode the contained envelope

				envelope := &Envelope{}

				err := protobuf.Decode(packet.Payload, envelope)
				if err != nil {
					fmt.Println(err)
				}

				err = whisper.handleEnvelope(&EnvelopeOrigin{Envelope: envelope, Origin: extPacket.SenderAddr})
				if err != nil {
					fmt.Println("\nWhisper: bad envelope received, peer " + extPacket.SenderAddr.String() + " will be disconnected")
					//fmt.Println(err)
					whisper.blacklist[extPacket.SenderAddr.String()] = struct{}{}
				}
			}
		}
	}
}

// GetEnvelope retrieves an envelope from its hash
func (whisper *Whisper) GetEnvelope(hash [32]byte) *EnvelopeOrigin {
	whisper.envelopes.Mutex.RLock()
	defer whisper.envelopes.Mutex.RUnlock()
	return whisper.envelopes.Envelopes[hash]
}

// GetMinPow returns the Pow value required
func (whisper *Whisper) GetMinPow() float64 {
	val, loaded := whisper.parameters.Load(minPowIdx)
	if !loaded {
		return DefaultMinimumPoW
	}
	return val.(float64)
}

// GetMinPowTolerated returns the pow tolerated for a limited time
func (whisper *Whisper) GetMinPowTolerated() float64 {
	val, exist := whisper.parameters.Load(minPowToleranceIdx)
	if !exist || val == nil {
		return DefaultMinimumPoW
	}
	return val.(float64)
}

// GetBloomFilter returns the aggregated bloom filter for all the topics of interest
func (whisper *Whisper) GetBloomFilter() []byte {
	value, loaded := whisper.parameters.Load(bloomFilterIdx)
	if !loaded {
		return nil
	}
	return value.([]byte)
}

// GetBloomFilterTolerated returns the bloom filter tolerated for a limited time
func (whisper *Whisper) GetBloomFilterTolerated() []byte {
	val, exist := whisper.parameters.Load(bloomFilterToleranceIdx)
	if !exist || val == nil {
		return nil
	}
	return val.([]byte)
}

// SetBloomFilter sets the new bloom filter
func (whisper *Whisper) SetBloomFilter(bloom []byte) error {

	if len(bloom) != BloomFilterSize {
		return fmt.Errorf("invalid bloom filter GetSize")
	}

	whisper.parameters.Store(bloomFilterIdx, bloom)

	wPacket := &gossiper.WhisperStatus{Code: bloomFilterExCode, Bloom: bloom}
	whisper.gossiper.SendWhisperStatus(wPacket)

	go func() {
		// let peers receive notification
		time.Sleep(time.Duration(DefaultSyncAllowance) * time.Second)
		whisper.parameters.Store(bloomFilterToleranceIdx, bloom)
	}()

	return nil
}

// SetMinPoW sets the minimal PoW required by this node
func (whisper *Whisper) SetMinPoW(pow float64) error {
	if pow < 0.0 {
		return fmt.Errorf("invalid pow")
	}

	whisper.parameters.Store(minPowIdx, pow)

	wPacket := &gossiper.WhisperStatus{Code: powRequirementCode, Pow: pow}
	whisper.gossiper.SendWhisperStatus(wPacket)

	go func() {
		// let peers receive notification
		time.Sleep(time.Duration(DefaultSyncAllowance) * time.Second)
		whisper.parameters.Store(minPowToleranceIdx, pow)
	}()

	return nil
}

// updateBloomFilter recomputes bloom filter
func (whisper *Whisper) updateBloomFilter(f *Filter) {
	aggregate := make([]byte, BloomFilterSize)
	for _, t := range f.Topics {
		top := ConvertBytesToTopic(t)
		b := ConvertTopicToBloom(top)
		aggregate = AggregateBloom(aggregate, b)
	}

	//fmt.Println(whisper.GetBloomFilter())
	//fmt.Println(aggregate)

	if !CheckFilterMatch(whisper.GetBloomFilter(), aggregate) {
		aggregate = AggregateBloom(whisper.GetBloomFilter(), aggregate)
		whisper.SetBloomFilter(aggregate)
	}
}

// handleEnvelope handles a new envelope (from peer or from me)
func (whisper *Whisper) handleEnvelope(envelopeOrigin *EnvelopeOrigin) error {
	now := uint32(time.Now().Unix())

	envelope := envelopeOrigin.Envelope

	isInterestingForPeers := false
	for peer, status := range whisper.routingHandler.peerStatus {
		if peer != envelopeOrigin.Origin.String() && CheckFilterMatch(status.Bloom, envelope.GetBloom()) && envelope.GetPow() >= status.Pow {
			isInterestingForPeers = true
			break
		}
	}

	var err error

	sent := envelope.Expiry - envelope.TTL

	if sent > now {
		if sent-DefaultSyncAllowance > now {
			err = fmt.Errorf("envelope created in the future")
		}
		envelope.computePow(sent - now + 1)
	}

	if envelope.Expiry < now {
		if envelope.Expiry+DefaultSyncAllowance*2 < now {
			err = fmt.Errorf("very old message")
		}
		fmt.Println("\nWhisper: expired envelope dropped")
	}

	if uint32(envelope.GetSize()) > MaxMessageSize {
		err = fmt.Errorf("huge messages are not allowed")
	}

	if envelope.GetPow() < whisper.GetMinPow() {
		if envelope.GetPow() < whisper.GetMinPowTolerated() {
			err = fmt.Errorf("\nWhisper: envelope with low pow received: " + fmt.Sprint(envelope.GetPow()))
		}
	}

	if !CheckFilterMatch(whisper.GetBloomFilter(), envelope.GetBloom()) {
		if !CheckFilterMatch(whisper.GetBloomFilterTolerated(), envelope.GetBloom()) {
			err = fmt.Errorf("\nWhisper: envelope does not match bloom filter")
		}
	}

	if err != nil && !isInterestingForPeers {
		return err
	}

	hash := envelope.GetHash()

	whisper.envelopes.Mutex.Lock()

	_, loaded := whisper.envelopes.Envelopes[hash]
	if !loaded {
		whisper.envelopes.Envelopes[hash] = envelopeOrigin
		go func(e *Envelope) {
			for len(whisper.messageQueue) >= messageQueueLimit {
				time.Sleep(time.Second)
			}
			whisper.messageQueue <- envelope
		}(envelope)
	} else {
		//fmt.Println("\nWhisper: envelope already in cache")
	}

	whisper.envelopes.Mutex.Unlock()

	return nil
}

// processQueue process all incoming accepted messages
func (whisper *Whisper) processQueue() {
	var e *Envelope
	for {
		select {
		case <-whisper.quit:
			return

		case e = <-whisper.messageQueue:
			whisper.filters.NotifySubscribers(e)
		}
	}
}

// updateEnvelopes periodically broadcast and flush envelopes
func (whisper *Whisper) updateEnvelopes() {
	expire := time.NewTicker(expirationTimer)
	defer expire.Stop()
	transmit := time.NewTicker(broadcastTimer)
	defer transmit.Stop()

	for {
		select {
		case <-expire.C:
			whisper.removeExpiredEnvelopes()

		case <-transmit.C:
			whisper.broadcastMessages()

		case <-whisper.quit:
			return
		}
	}
}

// broadcastMessages broadcast messages not expired yet
func (whisper *Whisper) broadcastMessages() {
	envelopes := whisper.Envelopes()
	for _, env := range envelopes {
		whisper.forwardEnvelope(env)
	}
}

// removeExpiredEnvelopes removes expired envelopes
func (whisper *Whisper) removeExpiredEnvelopes() {
	whisper.envelopes.Mutex.Lock()
	defer whisper.envelopes.Mutex.Unlock()

	now := uint32(time.Now().Unix())
	for hash, env := range whisper.envelopes.Envelopes {
		if env.Envelope.Expiry < now {
			delete(whisper.envelopes.Envelopes, hash)
		}
	}
}

// Envelopes retrieves all the envelopes
func (whisper *Whisper) Envelopes() []*EnvelopeOrigin {
	whisper.envelopes.Mutex.RLock()
	defer whisper.envelopes.Mutex.RUnlock()

	array := make([]*EnvelopeOrigin, 0, len(whisper.envelopes.Envelopes))
	for _, envelope := range whisper.envelopes.Envelopes {
		array = append(array, envelope)
	}
	return array
}
