package storage

import (
	"crypto/rand"
	"strings"
	"sync"
	"time"
)

// Constants used by DStore
const (
	DStoreSendBufferSize    int = 64
	DStoreReceiveBufferSize int = 64

	TypeBasicFile byte = 0
	TypeMetaFile  byte = 1

	HashTypeSha224     byte = 0
	HashTypeSha256     byte = 1
	HashTypeSha384     byte = 2
	HashTypeSha512     byte = 3
	HashTypeSha512_224 byte = 4
	HashTypeSha512_256 byte = 5

	ChunkSize int = 4096

	ReplyTimeout  uint64 = 2
	NanoSecPerSec uint64 = 1000000000

	RePublishTimeout uint64 = 60

	DStoreIDSize int = 8
)

// HashLength is a array of hash lengths in bytes for supported hash types
var HashLength [6]int = [6]int{28, 32, 48, 64, 28, 32}

// DStore represents a distributed storage component for a node.
// The implementation is based on Kademlia.
type DStore struct {
	// parameters
	replicationParam int
	concurrencyParam int

	dStoreID   []byte
	commonName string
	address    string

	sendChannel    chan *MessageAddrPair
	receiveChannel chan *MessageAddrPair

	routingTable     *RoutingTable
	chunkStore       *ChunkStore
	rpcReceiverStore *RPCReceiverStore

	displayLogs []string

	logMutex sync.Mutex
}

// NewDStore creates a new DStore for given node and parameters
func NewDStore(name string, addr string, replicationParam int, concurrencyParam int) *DStore {
	dstore := &DStore{replicationParam: replicationParam, concurrencyParam: concurrencyParam}

	dstore.commonName = name
	dstore.address = addr
	dstore.dStoreID = GetDStoreIDForName(addr)

	dstore.sendChannel = make(chan *MessageAddrPair, DStoreSendBufferSize)
	dstore.receiveChannel = make(chan *MessageAddrPair, DStoreReceiveBufferSize)

	dstore.routingTable = NewRoutingTable(dstore)
	dstore.chunkStore = NewChunkStore(dstore)
	dstore.rpcReceiverStore = NewRPCReceiverStore(dstore)

	dstore.logMessage("Starting up. My ID %s", str(dstore.dStoreID))
	return dstore
}

// Init initializes a DStore.
func (ds *DStore) Init(contacts []string) {
	for _, c := range contacts {
		r := &RoutingEntry{Address: c, ID: GetDStoreIDForName(c)}
		ds.routingTable.Update(r)
	}
	// try to populate the routing table
	go ds.populateRoutingTable()

	go ds.handleIncomingMessages()
}

// populateRoutingTable attempt to populate the initial routing table entries
func (ds *DStore) populateRoutingTable() {
	ds.NodeLookUp(ds.dStoreID)

	for i := DStoreIDSize - 1; i >= 0; i-- {
		randID := make([]byte, DStoreIDSize)
		rand.Read(randID)
		for j := DStoreIDSize - 1; j >= i; j-- {
			randID[j] = ds.dStoreID[j]
		}
		ds.NodeLookUp(randID)
	}
}

// GetReceiveChannel returns the channel used by the DStore to receive incoming messages.
// The caller should then send any incoming DStoreMessage to the returned channel.
// The returned channel can be blocking, so the caller should ideally send any message in a separate go routine.
func (ds *DStore) GetReceiveChannel() chan *MessageAddrPair {
	return ds.receiveChannel
}

// GetSendChannel returns the channel used by the DStore to send outgoing messages.
// The caller should read the returned channel and send the messages to its destination using any method.
func (ds *DStore) GetSendChannel() chan *MessageAddrPair {
	return ds.sendChannel
}

// PingNode pings a node synchronously and returns true if a reply is received within ReplyTimeout seconds.
func (ds *DStore) PingNode(addr string) bool {
	c := ds.sendAsyncRequest(NewPingRequest(), addr)
	select {
	case m := <-c:
		return m != nil
	case <-time.After(time.Duration(ReplyTimeout * NanoSecPerSec)):
		return false
	}
}

// RetrieveFile ...
func (ds *DStore) RetrieveFile(metaHash []byte) []byte {
	fileType := metaHash[0]
	hashType := metaHash[1]
	hashBlock := 2 + HashLength[int(hashType)]
	res := make([]byte, 0)
	for i := 0; i < len(metaHash); i += hashBlock {
		temp, _ := ds.ValueLookUp(metaHash[i : i+hashBlock])
		if temp == nil {
			return nil
		}
		res = append(res, temp...)
	}
	if fileType == TypeMetaFile {
		return ds.RetrieveFile(res)
	}
	return res
}

// StoreFile ...
func (ds *DStore) StoreFile(data []byte, hashType byte, fileType byte) []byte {
	hashBlockLen := 2 + HashLength[hashType]
	metaFile := make([]byte, 0)
	for i := 0; i < len(data); i += ChunkSize {
		chunk := data[i:Min(i+ChunkSize, len(data))]
		chunkHash := GetHash(chunk, hashType)
		metaFile = append(metaFile, fileType, hashType)
		metaFile = append(metaFile, chunkHash...)
		ds.storeChunk(metaFile[(i/ChunkSize)*hashBlockLen:((i/ChunkSize)+1)*hashBlockLen], chunk)
	}

	if len(metaFile) < ChunkSize {
		tempHash := GetHash(metaFile, hashType)
		metaHash := make([]byte, 0)
		metaHash = append(metaHash, TypeMetaFile, hashType)
		metaHash = append(metaHash, tempHash...)
		ds.storeChunk(metaHash, metaFile)
		return metaHash
	}

	return ds.StoreFile(metaFile, hashType, TypeMetaFile)
}

// storeChunk stores a given chunk in the replicationParam many node that are closest to the chunk hash.
func (ds *DStore) storeChunk(key []byte, chunk []byte) {
	lookUpID := key[2 : 2+DStoreIDSize]
	nodes := ds.NodeLookUp(lookUpID)

	resultChannel := make(chan *MessageAddrPair)

	for _, n := range nodes {
		msg := NewStoreRequest(key, chunk, 0, 0)
		c := ds.sendAsyncRequest(msg, n.Address)
		go func() {
			select {
			case m := <-c:
				resultChannel <- m
			case <-time.After(time.Duration(ReplyTimeout * NanoSecPerSec)):
				resultChannel <- nil
			}
		}()
	}

	ack := make([]string, 0)
	for i := 0; i < len(nodes); i++ {
		m := <-resultChannel
		if m == nil {
			continue
		}
		ack = append(ack, m.Address)
	}
	ds.logMessage("Chunk with hash %s stored in {%s}", str(key), strings.Join(ack, ", "))
}

// sendAsyncRply sends an RPC reply to addr.
func (ds *DStore) sendAsyncReply(msg *DStoreMessage, addr string) {
	msg.SourceID = ds.dStoreID
	go func() {
		ds.sendChannel <- &MessageAddrPair{Message: msg, Address: addr}
	}()
}

// sendAsyncRequest sends an RPC request to addr and returns a channel from which to expect the reply.
func (ds *DStore) sendAsyncRequest(msg *DStoreMessage, addr string) chan *MessageAddrPair {
	msg.SourceID = ds.dStoreID
	msg.RPCID = GenerateRandomRPCID(DStoreIDSize)

	replyChannel := make(chan *MessageAddrPair)
	ds.rpcReceiverStore.Put(msg.RPCID, replyChannel)

	go func() {
		ds.sendChannel <- &MessageAddrPair{Message: msg, Address: addr}
	}()

	return replyChannel
}

// GetAllChunkKeys returns all the keys stored in this storage instance.
func (ds *DStore) GetAllChunkKeys() []string {
	return ds.chunkStore.allChunks
}

// GetDisplayLogs returns the set of all log messages.
func (ds *DStore) GetDisplayLogs() []string {
	return ds.displayLogs
}
