// Contributors: Dewmini Marakkalage

package storage

// handleIncomingMessages multiplexes incoming messages to the appropriate handler.
func (ds *DStore) handleIncomingMessages() {
	for msgAddrPair := range ds.receiveChannel {
		msg, addr := msgAddrPair.Message, msgAddrPair.Address
		// update routing table
		re := &RoutingEntry{ID: msg.SourceID, Address: addr}
		go func() {
			ds.routingTable.Update(re)
		}()

		switch {
		case msg.PingRequest != nil:
			ds.invokePingRPC(msgAddrPair)
		case msg.FindNodeRequest != nil:
			ds.invokeFindNodeRPC(msgAddrPair)
		case msg.FindValueRequest != nil:
			ds.invokeFindValueRPC(msgAddrPair)
		case msg.StoreRequest != nil:
			ds.invokeStoreRPC(msgAddrPair)
		default:
			ds.handleRPCReply(msgAddrPair)
		}
	}
}

// invokePingRPC is called when a PingRequest message is received.
func (ds *DStore) invokePingRPC(msgAddrPair *MessageAddrPair) {
	msg, addr := msgAddrPair.Message, msgAddrPair.Address
	rpcID := msg.RPCID
	reply := &DStoreMessage{RPCID: rpcID, PingReply: &PingReply{}}
	ds.sendAsyncReply(reply, addr)
}

// invokeStoreRPC is called when a StoreRequest message is received.
func (ds *DStore) invokeStoreRPC(msgAddrPair *MessageAddrPair) {
	msg, addr := msgAddrPair.Message, msgAddrPair.Address
	rpcID, key, value := msg.RPCID, msg.StoreRequest.Key, msg.StoreRequest.Value

	ds.chunkStore.Put(key, value)

	reply := &DStoreMessage{RPCID: rpcID, StoreReply: &StoreReply{Key: key, Value: value}}
	ds.sendAsyncReply(reply, addr)
}

// invokeFindNodeRPC is called when a FindNodeRequest is received.
func (ds *DStore) invokeFindNodeRPC(msgAddrPair *MessageAddrPair) {
	msg, addr := msgAddrPair.Message, msgAddrPair.Address
	rpcID, toFind := msg.RPCID, msg.FindNodeRequest.Target

	reply := &DStoreMessage{RPCID: rpcID, FindNodeReply: &FindNodeReply{Nodes: ds.routingTable.FindKClosest(toFind, ds.replicationParam)}}
	ds.sendAsyncReply(reply, addr)
}

// invokeFindValueRPC is called when a FindValueRequest is received.
func (ds *DStore) invokeFindValueRPC(msgAddrPair *MessageAddrPair) {
	msg, addr := msgAddrPair.Message, msgAddrPair.Address
	rpcID, toFind := msg.RPCID, msg.FindValueRequest.Key

	val, ok := ds.chunkStore.Get(toFind)
	if ok {
		reply := &DStoreMessage{RPCID: rpcID, FindValueReply: &FindValueReply{Key: toFind, Value: val}}
		ds.sendAsyncReply(reply, addr)
		return
	}

	nodeToFind := toFind[2 : 2+DStoreIDSize]
	reply := &DStoreMessage{RPCID: rpcID, FindValueReply: &FindValueReply{Key: toFind, Nodes: ds.routingTable.FindKClosest(nodeToFind, ds.replicationParam)}}
	ds.sendAsyncReply(reply, addr)
}

// handRPCReply sends RPC replies via the designated channels.
func (ds *DStore) handleRPCReply(msgAddrPair *MessageAddrPair) {
	c, ok := ds.rpcReceiverStore.Get(msgAddrPair.Message.RPCID)
	if !ok {
		return
	}
	go func() {
		select {
		case c <- msgAddrPair:
		default:
		}
	}()
}
