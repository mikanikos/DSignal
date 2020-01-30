// Contributors: Andrea Piccione

package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// BlockchainHandler struct
type BlockchainHandler struct {
	myTime uint32

	tlcAckChan     chan *TLCAck
	tlcConfirmChan chan bool

	tlcStatus sync.Map

	confirmations sync.Map
	messageSeen   sync.Map

	committedHistory sync.Map

	topBlockchainHash [32]byte
	previousBlockHash [32]byte

	blockchainLogs chan string
}

// NewBlockchainHandler create new blockchain handler
func NewBlockchainHandler() *BlockchainHandler {
	return &BlockchainHandler{
		myTime: 0,
		// process incoming tlc acks
		tlcAckChan: make(chan *TLCAck, maxChannelSize),
		// dummy channel to notify for confirmations
		tlcConfirmChan: make(chan bool, maxChannelSize),
		// track round of peers based on confirmed messages
		tlcStatus: sync.Map{},
		// confirmations saved by round
		confirmations: sync.Map{},
		// tlc messages seen saved by round
		messageSeen: sync.Map{},
		// blockchain
		committedHistory: sync.Map{},
		// top of the blockchain
		topBlockchainHash: [32]byte{},
		// hash of previous block
		previousBlockHash: [32]byte{},
		// blockchain logs (prints) for gui
		blockchainLogs: make(chan string, maxChannelSize),
	}
}

// SafeTLCMessagesMap struct
type SafeTLCMessagesMap struct {
	OriginMessage map[string]*TLCMessage
	Mutex         sync.RWMutex
}

// SafeTLCMap struct
type SafeTLCMap struct {
	MessagedIDs map[int]bool
	Mutex       sync.RWMutex
}

// create tx block and start finding an agreement with others
func (gossiper *Gossiper) createAndPublishTxBlock(fileMetadata *FileMetadata) {

	// create tx block
	tx := TxPublish{Name: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, Size: fileMetadata.Size}
	block := BlockPublish{Transaction: tx, PrevHash: gossiper.blockchainHandler.previousBlockHash}
	extPacket := gossiper.createTLCMessage(block, -1, rand.Float32())

	// if no simple gossip with confirmation, send it to client block buffer
	if hw3ex2Mode && !hw3ex3Mode && !hw3ex4Mode {
		gossiper.gossipWithConfirmation(extPacket, false)
	} else {
		// send data to round block queue
		go func(e *ExtendedGossipPacket) {
			PacketChannels["clientBlock"] <- e
		}(extPacket)
	}
}

// process incoming blocks from client (namely new files to be indexed) and choose to use qsc or tlc according to flags
func (gossiper *Gossiper) processClientBlocks() {
	for extPacket := range PacketChannels["clientBlock"] {
		if hw3ex4Mode {
			gossiper.qscRound(extPacket)
		} else {
			gossiper.tlcRound(extPacket)
		}

		if debug {
			fmt.Println("Round completed")
		}
	}
}

// process tlc message
func (gossiper *Gossiper) handleTLCMessage() {
	for extPacket := range PacketChannels["tlcCausal"] {

		// get round of the message
		messageRound := gossiper.getMessageOriginalRound(extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.ID, extPacket.Packet.TLCMessage.Confirmed)

		if debug {
			if extPacket.Packet.TLCMessage.Confirmed > -1 {
				fmt.Println(fmt.Sprint(extPacket.Packet.TLCMessage.Confirmed) + " of " + extPacket.Packet.TLCMessage.Origin + " is at round " + fmt.Sprint(messageRound))
			} else {
				fmt.Println(fmt.Sprint(extPacket.Packet.TLCMessage.ID) + " of " + extPacket.Packet.TLCMessage.Origin + " is at round " + fmt.Sprint(messageRound))
			}
		}

		// Ack message, depending on flags combination
		if !hw3ex4Mode || gossiper.blockchainHandler.isTxBlockValid(extPacket.Packet.TLCMessage.TxBlock) {
			if !hw3ex3Mode || ackAllMode || uint32(messageRound) >= gossiper.blockchainHandler.myTime {
				if !(extPacket.Packet.TLCMessage.Confirmed > -1) {

					// save messeage for qsc
					if hw3ex4Mode {
						gossiper.blockchainHandler.saveMessageSeen(messageRound, extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage)
					}

					// send ack for tlc
					privatePacket := &TLCAck{Origin: gossiper.Name, ID: extPacket.Packet.TLCMessage.ID, Destination: extPacket.Packet.TLCMessage.Origin, HopLimit: uint32(hopLimit)}
					if hw3ex2Mode || hw3ex3Mode {
						fmt.Println("SENDING ACK origin " + extPacket.Packet.TLCMessage.Origin + " ID " + fmt.Sprint(extPacket.Packet.TLCMessage.ID))
					}
					go gossiper.forwardPrivateMessage(&GossipPacket{Ack: privatePacket}, &privatePacket.HopLimit, privatePacket.Destination)
				}
			}
		}

		// consider confirmed messages only for confirmations
		if (hw3ex3Mode || hw3ex4Mode) && extPacket.Packet.TLCMessage.Confirmed != -1 {
			if gossiper.checkForCausalProperty(extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.VectorClock, extPacket.Packet.TLCMessage.Confirmed, extPacket.SenderAddr) {

				// check if I can take this confirmation, otherwise process it again later

				if debug {
					fmt.Println("Got confirm for " + fmt.Sprint(extPacket.Packet.TLCMessage.Confirmed) + " from " + extPacket.Packet.TLCMessage.Origin + " " + fmt.Sprint(extPacket.Packet.TLCMessage.Fitness))
				}

				// save confirmation of the round
				gossiper.blockchainHandler.saveConfirmation(messageRound, extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage)

				// notify active goroutine of a possible change in confimations
				go func() {
					gossiper.blockchainHandler.tlcConfirmChan <- true
				}()

			} else {
				// re-send it to the queue after timeout
				go func(ext *ExtendedGossipPacket) {
					time.Sleep(time.Duration(tlcQueueTimeout) * time.Second)
					PacketChannels["tlcCausal"] <- ext
				}(extPacket)
			}
		}
	}
}

// save confirmations, needed for qsc and to know if we have a majority
func (blockchainHandler *BlockchainHandler) saveConfirmation(round uint32, origin string, tlc *TLCMessage) {

	currentRound := atomic.LoadUint32(&blockchainHandler.myTime)

	// do not take into consideration previous rounds
	if round >= currentRound {

		value, _ := blockchainHandler.confirmations.LoadOrStore(round, &SafeTLCMessagesMap{OriginMessage: make(map[string]*TLCMessage)})
		tlcMap := value.(*SafeTLCMessagesMap)

		tlcMap.Mutex.Lock()
		defer tlcMap.Mutex.Unlock()

		tlcMap.OriginMessage[origin] = tlc

		if debug {
			blockchainHandler.confirmations.Range(func(key interface{}, value interface{}) bool {
				roundI := key.(uint32)
				tlcM := value.(*SafeTLCMessagesMap)
				fmt.Print("Round " + fmt.Sprint(roundI) + ": ")
				for k, v := range tlcM.OriginMessage {
					fmt.Print(k + " " + fmt.Sprint(v.ID) + " | ")
				}
				fmt.Println()
				return true
			})
		}
	}
}

// save all messages seen in every round, it's useful for qsc in order to know if there's been a message with higher fitness
func (blockchainHandler *BlockchainHandler) saveMessageSeen(round uint32, origin string, tlc *TLCMessage) {
	value, _ := blockchainHandler.messageSeen.LoadOrStore(round, &SafeTLCMessagesMap{OriginMessage: make(map[string]*TLCMessage)})
	tlcMap := value.(*SafeTLCMessagesMap)

	tlcMap.Mutex.Lock()
	defer tlcMap.Mutex.Unlock()

	tlcMap.OriginMessage[origin] = tlc
}

// create tlc message with given paramters
func (gossiper *Gossiper) createTLCMessage(block BlockPublish, confirmedFlag int, fitness float32) *ExtendedGossipPacket {
	id := atomic.LoadUint32(&gossiper.gossipHandler.seqID)
	atomic.AddUint32(&gossiper.gossipHandler.seqID, uint32(1))

	tlcPacket := &TLCMessage{Origin: gossiper.Name, ID: id, TxBlock: block, VectorClock: gossiper.gossipHandler.myStatus.createMyStatusPacket(), Confirmed: confirmedFlag, Fitness: fitness}
	extPacket := &ExtendedGossipPacket{Packet: &GossipPacket{TLCMessage: tlcPacket}, SenderAddr: gossiper.ConnectionHandler.GossiperData.Address}

	// store message
	gossiper.gossipHandler.storeMessage(extPacket.Packet, gossiper.Name, id)

	if debug {
		fmt.Print("Creating id " + fmt.Sprint(id) + ": ")
		fmt.Println(tlcPacket.VectorClock.Want)
	}

	return extPacket
}

// get round origin of the message
func (gossiper *Gossiper) getMessageOriginalRound(origin string, id uint32, confirmedID int) uint32 {
	value, _ := gossiper.blockchainHandler.tlcStatus.LoadOrStore(origin, &SafeTLCMap{MessagedIDs: make(map[int]bool)})
	tlcMap := value.(*SafeTLCMap)

	tlcMap.Mutex.RLock()
	defer tlcMap.Mutex.RUnlock()

	// compute round of the message
	messageRound := uint32(0)

	// distinguish betweeen confirmed and unconfirmed
	if confirmedID > -1 {
		for key := range tlcMap.MessagedIDs {
			if confirmedID > key {
				messageRound++
			}
		}
	} else {
		for key := range tlcMap.MessagedIDs {
			if id > uint32(key) {
				messageRound++
			}
		}
	}

	return messageRound
}

// check if message can be accepted for the causal property
func (gossiper *Gossiper) checkForCausalProperty(origin string, receivedVC *StatusPacket, confirmedID int, address *net.UDPAddr) bool {

	// do not consider unconfirmed messages
	if confirmedID > -1 {
		// check vector clock, if v_other[] <= v_mine[] true otherwise false
		if !gossiper.gossipHandler.myStatus.checkIfINeedPeerStatus(receivedVC.Want) {

			if debug {
				fmt.Println("Vector clock OK!")
			}

			// update tlc status
			value, _ := gossiper.blockchainHandler.tlcStatus.LoadOrStore(origin, &SafeTLCMap{MessagedIDs: make(map[int]bool)})
			tlcMap := value.(*SafeTLCMap)

			tlcMap.Mutex.Lock()
			tlcMap.MessagedIDs[confirmedID] = true
			tlcMap.Mutex.Unlock()

			return true
		}

		// send status in order to get missing packets (should already be done by gossipHandler but this should be faster and does not cost a lot)
		statusToSend := gossiper.gossipHandler.myStatus.createMyStatusPacket()
		gossiper.ConnectionHandler.SendPacket(&GossipPacket{Status: statusToSend}, address)
	}

	return false
}
