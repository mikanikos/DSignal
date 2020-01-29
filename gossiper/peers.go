package gossiper

import (
	"net"
	"strings"
	"sync"
)

// PeersData struct
type PeersData struct {
	Peers []*net.UDPAddr
	Size  uint64
	Mutex sync.RWMutex
}

// create PeersData
func createPeersData(peers string, size uint64) *PeersData {

	// split list of peers addresses only if it's not empty in order to avoid problems with peers
	peersList := make([]string, 0)
	if peers != "" {
		peersList = strings.Split(peers, ",")
	}

	// resolve peers addresses given
	peersAddresses := make([]*net.UDPAddr, 0)
	for _, peer := range peersList {
		addressPeer, err := net.ResolveUDPAddr("udp4", peer)
		if err == nil {
			peersAddresses = append(peersAddresses, addressPeer)
		}
	}

	return &PeersData{Peers: peersAddresses, Size: size}
}

// AddPeer to peers list if not present
func (gossiper *Gossiper) AddPeer(peer *net.UDPAddr) {

	if peer.String() == gossiper.ConnectionHandler.GossiperData.Address.String() {
		return
	}

	gossiper.PeersData.Mutex.Lock()
	defer gossiper.PeersData.Mutex.Unlock()
	contains := false
	for _, p := range gossiper.PeersData.Peers {
		if p.String() == peer.String() {
			contains = true
			break
		}
	}
	if !contains {
		gossiper.PeersData.Peers = append(gossiper.PeersData.Peers, peer)
	}
}

// GetPeer given the string address
func (gossiper *Gossiper) GetPeerFromString(addr string) *net.UDPAddr {
	gossiper.PeersData.Mutex.RLock()
	defer gossiper.PeersData.Mutex.RUnlock()
	for _, p := range gossiper.PeersData.Peers {
		if p.String() == addr {
			return p
		}
	}
	return nil
}

// GetPeers in concurrent environment
func (gossiper *Gossiper) GetPeers() []*net.UDPAddr {
	gossiper.PeersData.Mutex.RLock()
	defer gossiper.PeersData.Mutex.RUnlock()
	peerCopy := make([]*net.UDPAddr, len(gossiper.PeersData.Peers))
	copy(peerCopy, gossiper.PeersData.Peers)
	return peerCopy
}
