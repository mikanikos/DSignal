package gossiper

import (
	"encoding/hex"
	"fmt"
	"sync/atomic"
	"time"
)

// start tlc round
func (gossiper *Gossiper) tlcRound(extPacket *ExtendedGossipPacket) {
	gossiper.gossipWithConfirmation(extPacket, true)

	if debug {
		fmt.Println("Done tlc round")
	}
}

// gossip with confirmation, wait for confirmation flag means tlc
func (gossiper *Gossiper) gossipWithConfirmation(extPacket *ExtendedGossipPacket, waitConfirmations bool) {

	if debug {
		fmt.Println("Run gossip with confirmation")
	}

	if hw3ex2Mode {
		gossiper.printPeerMessage(extPacket, gossiper.GetPeers())
	}

	tlc := extPacket.Packet.TLCMessage

	// start rumor mongering the message
	go gossiper.StartRumorMongering(extPacket, gossiper.Name, tlc.ID)

	// if already got a majority of confirmations, I increment my round, send confirmation and return
	if waitConfirmations && gossiper.checkForConfirmationsMajority(tlc, false) {
		return
	}

	if stubbornTimeout > 0 {
		// initialize variables, witnesses (acks) used as a set
		witnesses := make(map[string]uint32)
		witnesses[gossiper.Name] = 0
		delivered := false
		gossiper.blockchainHandler.tlcAckChan = make(chan *TLCAck, maxChannelSize)

		// start timer
		timer := time.NewTicker(time.Duration(stubbornTimeout) * time.Second)
		defer timer.Stop()

		for {
			select {
			// if got ack for this message, record it
			case tlcAck := <-gossiper.blockchainHandler.tlcAckChan:

				if tlcAck.ID == tlc.ID && !delivered {
					witnesses[tlcAck.Origin] = 0

					// check if received a majority of acks
					if len(witnesses) > int(gossiper.PeersData.Size/2) {

						// create tlc confirmed message
						confirmedPacket := gossiper.createTLCMessage(tlc.TxBlock, int(tlc.ID), tlc.Fitness)
						gossiper.sendGossipWithConfirmation(confirmedPacket, witnesses)
						delivered = true

						// send confirmation
						if waitConfirmations {
							// save confirmation
							gossiper.blockchainHandler.saveConfirmation(atomic.LoadUint32(&gossiper.blockchainHandler.myTime), confirmedPacket.Packet.TLCMessage.Origin, confirmedPacket.Packet.TLCMessage)

							// notify that confirmationts threshold might have been reached
							go func() {
								gossiper.blockchainHandler.tlcConfirmChan <- true
							}()

						} else {
							return
						}
					}
				}

			// every time I get a confirmation, I check if threshold reached
			case <-gossiper.blockchainHandler.tlcConfirmChan:
				if waitConfirmations && gossiper.checkForConfirmationsMajority(tlc, delivered) {
					return
				}

			// periodically resend tlc message
			case <-timer.C:
				if hw3ex2Mode {
					gossiper.printPeerMessage(extPacket, gossiper.GetPeers())
				}
				go gossiper.StartRumorMongering(extPacket, gossiper.Name, tlc.ID)
			}
		}
	}
}

// prepare gossip with confirmation and rumor monger it
func (gossiper *Gossiper) sendGossipWithConfirmation(extPacket *ExtendedGossipPacket, witnesses map[string]uint32) {

	if hw3ex2Mode {
		printConfirmMessage(extPacket.Packet.TLCMessage.ID, witnesses)
	}

	// rumor momger it
	go gossiper.StartRumorMongering(extPacket, extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.ID)

	// send it to gui
	if !hw3ex4Mode {
		go func(b *BlockPublish) {
			gossiper.fileHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
		}(&extPacket.Packet.TLCMessage.TxBlock)
	}
}

// check for majority of confirmations and increment round
func (gossiper *Gossiper) checkForConfirmationsMajority(tlc *TLCMessage, delivered bool) bool {
	// get current round
	round := atomic.LoadUint32(&gossiper.blockchainHandler.myTime)

	// get confirmations for this round
	value, loaded := gossiper.blockchainHandler.confirmations.Load(round)

	if loaded {

		tlcMap := value.(*SafeTLCMessagesMap)

		tlcMap.Mutex.RLock()
		confirmations := tlcMap.OriginMessage
		tlcMap.Mutex.RUnlock()

		if debug {
			fmt.Println("Got: " + fmt.Sprint(len(confirmations)))
			fmt.Println(confirmations)
		}

		// check if got majority of confirmations and increment round in that case
		if len(confirmations) > int(gossiper.PeersData.Size/2) {

			atomic.AddUint32(&gossiper.blockchainHandler.myTime, uint32(1))
			gossiper.printRoundMessage(gossiper.blockchainHandler.myTime, confirmations)

			// if not sent confirmation yet, do it now
			if !delivered {
				confirmedPacket := gossiper.createTLCMessage(tlc.TxBlock, int(tlc.ID), tlc.Fitness)
				gossiper.sendGossipWithConfirmation(confirmedPacket, getIDForConfirmations(confirmations))
			}

			return true
		}
	}

	return false
}
