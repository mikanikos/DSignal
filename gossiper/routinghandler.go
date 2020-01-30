// Authors: Andrea Piccione

package gossiper

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// RoutingHandler struct
type RoutingHandler struct {
	// routing table peer->nexthop
	routingTable map[string]*net.UDPAddr
	// track current last id (just an optimization in order to not iterate on the message storage every time)
	originLastID *VectorClock
	mutex        sync.RWMutex
}

// NewRoutingHandler create new routing handler
func NewRoutingHandler() *RoutingHandler {
	return &RoutingHandler{
		routingTable: make(map[string]*net.UDPAddr),
		originLastID: &VectorClock{Entries: make(map[string]uint32)},
	}
}

// StartRouteRumormongering with the specified timer
func (gossiper *Gossiper) startRouteRumormongering() {

	if routeRumorTimeout > 0 {

		// create new rumor message
		extPacket := gossiper.CreateRumorMessage("")

		// broadcast it initially in order to start well
		go gossiper.ConnectionHandler.BroadcastToPeers(extPacket, gossiper.GetPeers())

		// start timer
		timer := time.NewTicker(time.Duration(routeRumorTimeout) * time.Second)
		for {
			select {
			// rumor monger rumor at each timeout
			case <-timer.C:
				// create new rumor message
				extPacket := gossiper.CreateRumorMessage("")

				// start rumormongering the message
				go gossiper.StartRumorMongering(extPacket, gossiper.Name, extPacket.Packet.Rumor.ID)
			}
		}
	}
}

// update routing table based on packet data
func (routingHandler *RoutingHandler) updateRoutingTable(origin, textPacket string, idPacket uint32, address *net.UDPAddr) {

	routingHandler.mutex.Lock()
	defer routingHandler.mutex.Unlock()

	// if new packet with higher id, update table
	if routingHandler.updateLastOriginID(origin, idPacket) {

		if textPacket != "" {
			if hw2 {
				fmt.Println("DSDV " + origin + " " + address.String())
			}
		}

		// update
		routingHandler.routingTable[origin] = address

		if debug {
			fmt.Println("Routing table updated")
		}
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

// forward private message
func (gossiper *Gossiper) forwardPrivateMessage(packet *GossipPacket, hopLimit *uint32, destination string) {

	// check if hop limit is valid, decrement it if ok
	if *hopLimit > 0 {
		*hopLimit = *hopLimit - 1

		// get routing table entry for the origin
		gossiper.routingHandler.mutex.RLock()
		addressInTable, isPresent := gossiper.routingHandler.routingTable[destination]
		gossiper.routingHandler.mutex.RUnlock()

		// send packet if address is present
		if isPresent {
			gossiper.ConnectionHandler.SendPacket(packet, addressInTable)
		}
	}
}

// GetOrigins in concurrent environment
func (gossiper *Gossiper) GetOrigins() []string {
	gossiper.routingHandler.mutex.RLock()
	defer gossiper.routingHandler.mutex.RUnlock()

	origins := make([]string, len(gossiper.routingHandler.originLastID.Entries))
	i := 0
	for k := range gossiper.routingHandler.originLastID.Entries {
		origins[i] = k
		i++
	}
	return origins
}
