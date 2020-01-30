// Authors: Dewmini Marakkalage

package storage

import (
	"encoding/hex"
	"sync"
	"time"
)

// ChunkStore ...
type ChunkStore struct {
	dStore             *DStore
	chunkMap           map[string][]byte
	chunkExpiryTime    map[string]uint64
	chunkRepublishTime map[string]uint64
	mutex              sync.RWMutex
	allChunks          []string
}

// NewChunkStore creates a new ChunkStore
func NewChunkStore(dStore *DStore) *ChunkStore {
	return &ChunkStore{dStore: dStore, chunkMap: make(map[string][]byte), allChunks: make([]string, 0)}
}

// Put stores a new chunk in this chunk store.
func (cs *ChunkStore) Put(key []byte, value []byte) {
	keyStr := hex.EncodeToString(key)
	cs.mutex.Lock()
	defer cs.mutex.Unlock()
	if _, ok := cs.chunkMap[keyStr]; !ok {
		cs.allChunks = append(cs.allChunks, keyStr)
		go func() { // TODO make this more efficient
			t := time.NewTicker(time.Duration(RePublishTimeout * NanoSecPerSec))
			for {
				<-t.C
				cs.dStore.storeChunk(key, value)
			}
		}()
		cs.chunkMap[keyStr] = value
	}
}

// Get returns a chunk associated with a given key from this chunk store.
func (cs *ChunkStore) Get(key []byte) ([]byte, bool) {
	keyStr := hex.EncodeToString(key)
	cs.mutex.RLock()
	defer cs.mutex.RUnlock()
	val, ok := cs.chunkMap[keyStr]
	return val, ok
}
