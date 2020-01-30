// Contributors: Andrea Piccione

package gossiper

import "sync"

// VectorClock struct
type VectorClock struct {
	Entries map[string]uint32
	Mutex   sync.RWMutex
}

// update my vector clock based on current information of mapvalue
func (gossipHandler *GossipHandler) updateStatus(origin string, id uint32, mapValue *sync.Map) {

	gossipHandler.myStatus.Mutex.Lock()
	defer gossipHandler.myStatus.Mutex.Unlock()

	value, peerExists := gossipHandler.myStatus.Entries[origin]
	maxID := uint32(1)
	if peerExists {
		maxID = uint32(value)
	}

	// increment up to the maximum consecutive known message, i.e. least unknown message
	if maxID <= id {
		_, found := mapValue.Load(maxID)
		for found {
			maxID++
			_, found = mapValue.Load(maxID)
			gossipHandler.myStatus.Entries[origin] = maxID
		}
	}
}

// get gossip packet from peer status
func (gossipHandler *GossipHandler) getPacketFromPeerStatus(ps PeerStatus) *GossipPacket {
	value, _ := gossipHandler.messageStorage.Load(ps.Identifier)
	idMessages := value.(*sync.Map)
	message, _ := idMessages.Load(ps.NextID)
	return message.(*GossipPacket)
}

// create status packet from current vector clock
func (status *VectorClock) createMyStatusPacket() *StatusPacket {

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	myStatus := make([]PeerStatus, 0)

	for origin, nextID := range status.Entries {
		myStatus = append(myStatus, PeerStatus{Identifier: origin, NextID: nextID})
	}

	statusPacket := &StatusPacket{Want: myStatus}
	return statusPacket
}

// compare peer status and current status and get an entry the other needs
func (status *VectorClock) getPeerStatusForPeer(otherStatus []PeerStatus) *PeerStatus {

	originIDMap := make(map[string]uint32)
	for _, elem := range otherStatus {
		originIDMap[elem.Identifier] = elem.NextID
	}

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	for origin, nextID := range status.Entries {
		id, isOriginKnown := originIDMap[origin]
		if !isOriginKnown {
			return &PeerStatus{Identifier: origin, NextID: 1}
		} else if nextID > id {
			return &PeerStatus{Identifier: origin, NextID: id}
		}
	}
	return nil
}

// check if I need anything from the other peer status
func (status *VectorClock) checkIfINeedPeerStatus(otherStatus []PeerStatus) bool {

	status.Mutex.RLock()
	defer status.Mutex.RUnlock()

	for _, elem := range otherStatus {
		id, isOriginKnown := status.Entries[elem.Identifier]
		if !isOriginKnown {
			return true
		} else if elem.NextID > id {
			return true
		}
	}
	return false
}

// MessageUniqueID struct
type MessageUniqueID struct {
	Origin string
	ID     uint32
}

// get listener for incoming status
func (gossipHandler *GossipHandler) getListenerForStatus(origin string, id uint32, peer string) (chan bool, bool) {
	msgChan, _ := gossipHandler.mongeringChannels.LoadOrStore(peer, &sync.Map{})
	channel, loaded := msgChan.(*sync.Map).LoadOrStore(MessageUniqueID{Origin: origin, ID: id}, make(chan bool, maxChannelSize))
	return channel.(chan bool), loaded
}

// delete listener for incoming status
func (gossipHandler *GossipHandler) deleteListenerForStatus(origin string, id uint32, peer string) {
	msgChan, _ := gossipHandler.mongeringChannels.LoadOrStore(peer, &sync.Map{})
	msgChan.(*sync.Map).Delete(MessageUniqueID{Origin: origin, ID: id})
}

// notify listeners if status received satify condition
func (gossipHandler *GossipHandler) notifyListenersForStatus(extpacket *ExtendedGossipPacket) {
	msgChan, exists := gossipHandler.mongeringChannels.Load(extpacket.SenderAddr.String())
	if exists {
		msgChan.(*sync.Map).Range(func(key interface{}, value interface{}) bool {
			msg := key.(MessageUniqueID)
			channel := value.(chan bool)
			for _, ps := range extpacket.Packet.Status.Want {
				if ps.Identifier == msg.Origin && msg.ID < ps.NextID {
					go func(c chan bool) {
						c <- true
					}(channel)
				}
			}

			return true
		})
	}
}
