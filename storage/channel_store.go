package storage

import (
	"sync"
)

// RPCReceiverStore maps rpc IDs with associated receiver channels.
type RPCReceiverStore struct {
	dStore    *DStore
	rpcRecMap map[string]chan *MessageAddrPair
	mutex     sync.RWMutex
}

// NewRPCReceiverStore returns a new RPCReceiverStore.
func NewRPCReceiverStore(dStore *DStore) *RPCReceiverStore {
	return &RPCReceiverStore{dStore: dStore, rpcRecMap: make(map[string]chan *MessageAddrPair)}
}

// Put a given key-value pair in the store.
func (rs *RPCReceiverStore) Put(key string, channel chan *MessageAddrPair) {
	rs.mutex.Lock()
	defer rs.mutex.Unlock()
	rs.rpcRecMap[key] = channel
}

// Get the value associated with a given key from the store.
func (rs *RPCReceiverStore) Get(key string) (chan *MessageAddrPair, bool) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	channel, ok := rs.rpcRecMap[key]
	return channel, ok
}

// Delete the entry associated with a given key from the store.
func (rs *RPCReceiverStore) Delete(key string) {
	rs.mutex.RLock()
	defer rs.mutex.RUnlock()
	delete(rs.rpcRecMap, key)
}
