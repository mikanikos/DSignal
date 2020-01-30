// Authors: Dewmini Marakkalage

package storage

import (
	"encoding/hex"
	"sort"
	"sync"
)

// RoutingTable ...
type RoutingTable struct {
	dStore             *DStore
	PrefixMatchBuckets []*Bucket
}

// Bucket ...
type Bucket struct {
	EntryMap map[string]*RoutingEntryNode
	Head     *RoutingEntryNode
	Tail     *RoutingEntryNode
	Mutex    *sync.Mutex
}

// NewBucket creates a new bucket and initializes it.
func NewBucket() *Bucket {
	return &Bucket{EntryMap: make(map[string]*RoutingEntryNode), Head: nil, Tail: nil, Mutex: &sync.Mutex{}}
}

// RoutingEntry is a pair of an ID and its common name.
type RoutingEntry struct {
	ID      []byte
	Address string
}

// RoutingEntryNode wraps RoutingEntry objects to make a doubly linked list.
type RoutingEntryNode struct {
	prev, next *RoutingEntryNode
	entry      *RoutingEntry
}

// NewRoutingTable returns a new routing table.
func NewRoutingTable(ds *DStore) *RoutingTable {
	rt := &RoutingTable{
		dStore:             ds,
		PrefixMatchBuckets: make([]*Bucket, DStoreIDSize),
	}
	for i := 0; i < DStoreIDSize; i++ {
		rt.PrefixMatchBuckets[i] = NewBucket()
	}
	return rt
}

// FindKClosest finds the k nodes whose IDs are closest to the target in this routing table.
func (rt *RoutingTable) FindKClosest(target []byte, k int) []*RoutingEntry {
	type Pair struct {
		dist []byte
		re   *RoutingEntry
	}

	list := make([]*Pair, 0)

	for _, bucket := range rt.PrefixMatchBuckets {
		x := bucket.Head
		for x != nil {
			list = append(list, &Pair{dist: XorDistance(target, x.entry.ID), re: x.entry})
			x = x.next
		}
	}
	sort.Slice(list, func(i int, j int) bool { return Compare(list[i].dist, list[j].dist) < 0 })
	res := make([]*RoutingEntry, 0)
	for i := 0; i < k && i < len(list); i++ {
		res = append(res, list[i].re)
	}
	return res
}

// Update updates a routing table entry
func (rt *RoutingTable) Update(re *RoutingEntry) {
	maxPrefLen := MaxPrefixLen(rt.dStore.dStoreID, re.ID)
	if maxPrefLen < 0 || maxPrefLen >= 8*DStoreIDSize {
		return
	}

	bucket := rt.PrefixMatchBuckets[maxPrefLen]
	idStr := hex.EncodeToString(re.ID)

	bucket.Mutex.Lock()
	defer bucket.Mutex.Unlock()

	if entry, ok := bucket.EntryMap[idStr]; ok { // entry is already in the table
		if bucket.Tail != entry { // entry is not the tail
			// move the entry to the tail
			if entry.prev == nil { // entry is the head
				bucket.Head = entry.next
			} else {
				entry.prev.next = entry.next
			}
			if entry.next != nil {
				entry.next.prev = entry.prev
			}
			bucket.Tail.next = entry
			entry.prev = bucket.Tail
			entry.next = nil
			bucket.Tail = entry
		}
	} else { // entry not in the table
		if len(bucket.EntryMap) < rt.dStore.replicationParam {
			reNode := &RoutingEntryNode{entry: re, next: nil, prev: nil}
			if bucket.Head == nil {
				bucket.Head = reNode
				bucket.Tail = reNode
			} else {
				bucket.Tail.next = reNode
				reNode.prev = bucket.Tail
				bucket.Tail = reNode
			}
			bucket.EntryMap[idStr] = reNode
		} else {
			if !rt.dStore.PingNode(bucket.Head.entry.Address) {
				delete(bucket.EntryMap, hex.EncodeToString(bucket.Head.entry.ID))
				if bucket.Head.next != nil {
					bucket.Head.next.prev = nil
				}
				bucket.Head = bucket.Head.next
				reNode := &RoutingEntryNode{entry: re, next: nil, prev: nil}
				if bucket.Head == nil {
					bucket.Head = reNode
					bucket.Tail = reNode
				} else {
					bucket.Tail.next = reNode
					reNode.prev = bucket.Tail
					bucket.Tail = reNode
				}
				bucket.EntryMap[idStr] = reNode
			}
		}
	}
}
