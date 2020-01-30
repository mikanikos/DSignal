package whisper

import (
	//"fmt"
	"github.com/dedis/protobuf"
	"github.com/mikanikos/DSignal/gossiper"
	"math"
	"net"
	"sync"
	"time"
)

// Status is the struct that contains the main parameters of whisper
type Status struct {
	Pow   float64
	Bloom []byte
}

// RoutingHandler struct
type RoutingHandler struct {
	// peer -> aggregated bloom from that peer
	peerStatus map[string]*Status
	// track current last id to use the last updated information from peers
	originLastID *gossiper.VectorClock
	mutex        sync.RWMutex
}

// NewRoutingHandler create new routing handler
func NewRoutingHandler() *RoutingHandler {
	return &RoutingHandler{
		peerStatus: make(map[string]*Status),
		originLastID: &gossiper.VectorClock{Entries: make(map[string]uint32)},
	}
}

// updateRoutingTable according to status messages
func (routingHandler *RoutingHandler) updateRoutingTable(whisperStatus *gossiper.WhisperStatus, address *net.UDPAddr) {

	routingHandler.mutex.Lock()
	defer routingHandler.mutex.Unlock()

	// if new packet with higher id, updateEnvelopes table
	if routingHandler.updateLastOriginID(whisperStatus.Origin, whisperStatus.ID) {

		status, loaded := routingHandler.peerStatus[address.String()]
		if !loaded {
			routingHandler.peerStatus[address.String()] = &Status{}
			status, _ = routingHandler.peerStatus[address.String()]
		}

		if whisperStatus.Code == bloomFilterExCode || whisperStatus.Code == statusCode {
			if whisperStatus.Bloom != nil && len(whisperStatus.Bloom) == BloomFilterSize {
				if loaded {
					status.Bloom = AggregateBloom(whisperStatus.Bloom, status.Bloom)
				} else {
					status.Bloom = whisperStatus.Bloom
				}
				//fmt.Println("\nWhisper: routing table updated for BloomFilter, peer entry " + address.String())
			}
		}

		if whisperStatus.Code == powRequirementCode || whisperStatus.Code == statusCode {
			if !(math.IsInf(whisperStatus.Pow, 0) || math.IsNaN(whisperStatus.Pow) || whisperStatus.Pow < 0.0) {
				if loaded {
					status.Pow = math.Min(status.Pow, whisperStatus.Pow)
				} else {
					status.Pow = whisperStatus.Pow
				}
				//fmt.Println("\nWhisper: routing table updated for PoW, peer entry " + address.String())
			}
		}
		//fmt.Println("\nWhisper: routing table updated, peer entry " + address.String())
	}
}

// check if packet is new (has higher id) from that source, in that case it updates the table
func (routingHandler *RoutingHandler) updateLastOriginID(origin string, id uint32) bool {
	isNew := false

	originMaxID, loaded := routingHandler.originLastID.Entries[origin]
	if !loaded {
		routingHandler.originLastID.Entries[origin] = id
		isNew = true
	} else {
		if id > originMaxID {
			routingHandler.originLastID.Entries[origin] = id
			isNew = true
		}
	}

	return isNew
}

// forward envelope according to routing table and only to peers that might be interested
func (whisper *Whisper) forwardEnvelope(envOr *EnvelopeOrigin) {

	envelope := envOr.Envelope
	packetToSend, _ := protobuf.Encode(envelope)
	packet := &gossiper.GossipPacket{WhisperPacket: &gossiper.WhisperPacket{Code: messagesCode, Payload: packetToSend, Size: uint32(len(packetToSend))}}

	whisper.routingHandler.mutex.RLock()
	defer whisper.routingHandler.mutex.RUnlock()

	for peer, status := range whisper.routingHandler.peerStatus {
		if peer != envOr.Origin.String() {
			//fmt.Println(status.Bloom)
			//fmt.Println(envelope.GetBloom())
			//fmt.Println(CheckFilterMatch(status.Bloom, envelope.GetBloom()))
			//fmt.Println(fmt.Sprint(envelope.GetPow()) + " | " + fmt.Sprint(status.Pow))
			if CheckFilterMatch(status.Bloom, envelope.GetBloom()) && envelope.GetPow() >= status.Pow {
				//fmt.Println("Passed check")
				address := whisper.gossiper.GetPeerFromString(peer)
				if address != nil {
					//fmt.Println("\nWhisper: packet forwarded to peer " + address.String())
					whisper.gossiper.ConnectionHandler.SendPacket(packet, address)
				}
			}
		}
	}
}

// sendStatusPeriodically with the specified timer
func (whisper *Whisper) sendStatusPeriodically() {

	if statusTimer > 0 {

		wPacket := &gossiper.WhisperStatus{Code: statusCode, Pow: whisper.GetMinPow(), Bloom: whisper.GetBloomFilter()}
		whisper.gossiper.SendWhisperStatus(wPacket)

		//fmt.Println("Sent status")

		// start timer
		timer := time.NewTicker(statusTimer)
		for {
			select {
			// rumor monger rumor at each timeout
			case <-timer.C:
				//fmt.Println("Sent status")
				wPacket := &gossiper.WhisperStatus{Code: statusCode, Pow: whisper.GetMinPow(), Bloom: whisper.GetBloomFilter()}
				whisper.gossiper.SendWhisperStatus(wPacket)
			}
		}
	}
}
