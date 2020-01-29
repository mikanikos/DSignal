package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"time"

	"github.com/mikanikos/DSignal/helpers"
)

// StartAntiEntropy with the specified timer
func (gossiper *Gossiper) startAntiEntropy() {
	if antiEntropyTimeout > 0 {
		timer := time.NewTicker(time.Duration(antiEntropyTimeout) * time.Second)
		for {
			select {
			// at every timeout, send status to random peer
			case <-timer.C:
				peersCopy := gossiper.GetPeers()
				if len(peersCopy) != 0 {
					randomPeer := getRandomPeer(peersCopy)
					statusToSend := gossiper.gossipHandler.myStatus.createMyStatusPacket()
					gossiper.ConnectionHandler.SendPacket(&GossipPacket{Status: statusToSend}, randomPeer)
				}
			}
		}
	}
}

// rumor monger message
func (gossiper *Gossiper) StartRumorMongering(extPacket *ExtendedGossipPacket, origin string, id uint32) {
	peersWithRumor := []*net.UDPAddr{extPacket.SenderAddr}
	peers := gossiper.GetPeers()
	availablePeers := helpers.DifferenceString(peers, peersWithRumor)
	flipped := false

	if len(availablePeers) != 0 {
		// get random peer
		randomPeer := getRandomPeer(availablePeers)
		coin := 0
		// if coin is 0, continue; otherwise stop
		for coin == 0 {
			// send rumor with timeout to random peer
			statusReceived := gossiper.sendRumorWithTimeout(extPacket, origin, id, randomPeer)
			// if got status, flip coin
			if statusReceived {
				coin = rand.Int() % 2
				flipped = true
			}

			// if coin ok, get a new random peer
			if coin == 0 {
				peers := gossiper.GetPeers()
				peersWithRumor = []*net.UDPAddr{randomPeer}
				availablePeers := helpers.DifferenceString(peers, peersWithRumor)
				if len(availablePeers) == 0 {
					return
				}
				randomPeer = getRandomPeer(availablePeers)

				if flipped {
					if hw1 {
						fmt.Println("FLIPPED COIN sending rumor to " + randomPeer.String())
					}
					flipped = false
				}
			}
		}
	}
}

// send rumor and wait for status with timeout
func (gossiper *Gossiper) sendRumorWithTimeout(extPacket *ExtendedGossipPacket, origin string, id uint32, peer *net.UDPAddr) bool {

	// create listener for incoming status and delete it once done
	rumorChan, _ := gossiper.gossipHandler.getListenerForStatus(origin, id, peer.String())
	defer gossiper.gossipHandler.deleteListenerForStatus(origin, id, peer.String())

	if hw1 {
		fmt.Println("MONGERING with " + peer.String())
	}

	// send message
	gossiper.ConnectionHandler.SendPacket(extPacket.Packet, peer)

	// start timer
	timer := time.NewTicker(time.Duration(rumorTimeout) * time.Second)
	defer timer.Stop()

	for {
		// if got status, return true
		select {
		case <-rumorChan:
			if debug {
				fmt.Println("Got status")
			}
			return true

		// if timeout expired, return false
		case <-timer.C:
			if debug {
				fmt.Println("Timeout")
			}

			return false
		}
	}
}

// handle peer status for each peer
func (gossiper *Gossiper) handlePeerStatus(statusChannel chan *ExtendedGossipPacket) {
	for extPacket := range statusChannel {

		// notify status listeners
		go gossiper.gossipHandler.notifyListenersForStatus(extPacket)

		// get peer status that other might need
		toSend := gossiper.gossipHandler.myStatus.getPeerStatusForPeer(extPacket.Packet.Status.Want)
		if toSend != nil {
			packetToSend := gossiper.gossipHandler.getPacketFromPeerStatus(*toSend)
			gossiper.ConnectionHandler.SendPacket(packetToSend, extPacket.SenderAddr)
		} else {
			// check if I need something
			wanted := gossiper.gossipHandler.myStatus.checkIfINeedPeerStatus(extPacket.Packet.Status.Want)
			if wanted {
				statusToSend := gossiper.gossipHandler.myStatus.createMyStatusPacket()
				gossiper.ConnectionHandler.SendPacket(&GossipPacket{Status: statusToSend}, extPacket.SenderAddr)
			} else {
				// we're in sync
				if hw1 {
					fmt.Println("IN SYNC WITH " + extPacket.SenderAddr.String())
				}
			}
		}
	}
}
