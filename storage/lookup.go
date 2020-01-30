// Contributors: Dewmini Marakkalage

package storage

import (
	"fmt"
	"sort"
	//"strings"
	"time"
)

// NodeLookUp finds the replicationParam many nodes whose IDs are closest to a given ID.
func (ds *DStore) NodeLookUp(id []byte) []*RoutingEntry {
	if len(id) != DStoreIDSize {
		return make([]*RoutingEntry, 0)
	}

	//ds.logMessage("Node lookup for ID %s", str(id))

	type DistEntryPair struct {
		dist      []byte
		entry     *RoutingEntry
		contacted bool
		responded bool
	}

	nodes := ds.routingTable.FindKClosest(id, ds.replicationParam)

	entryMap := make(map[string]*DistEntryPair)
	entryList := make([]*DistEntryPair, len(nodes))

	for i, n := range nodes {
		entryList[i] = &DistEntryPair{dist: XorDistance(id, n.ID), entry: n}
		entryMap[str(entryList[i].entry.ID)] = entryList[i]
	}

	sort.Slice(entryList, func(i int, j int) bool { return Compare(entryList[i].dist, entryList[j].dist) < 0 })

	for {
		updated := false
		resChannel := make(chan *MessageAddrPair)
		j := 0
		for i := 0; i < len(entryList) && j < ds.concurrencyParam; i++ { // send to alpha closest nodes
			if entryList[i].contacted {
				continue
			}
			j++
			entryList[i].contacted = true
			channel := ds.sendAsyncRequest(NewFindNodeRequest(id), entryList[i].entry.Address)
			go func() {
				select {
				case m := <-channel:
					resChannel <- m
				case <-time.After(time.Duration(int64(ReplyTimeout) * 1000000000)):
					resChannel <- nil
				}
			}()
		}
		for k := 0; k < j; k++ {
			msgAddrPair := <-resChannel
			if msgAddrPair == nil {
				continue
			}

			msg := msgAddrPair.Message

			temp := entryMap[str(msg.SourceID)]
			if temp == nil {
				continue
			}
			temp.responded = true

			for _, re := range msg.FindNodeReply.Nodes {
				if _, ok := entryMap[str(re.ID)]; !ok { // a new node
					dist := XorDistance(id, re.ID)
					if len(entryList) < ds.replicationParam ||
						(len(entryList) >= ds.replicationParam && Compare(dist, entryList[ds.replicationParam-1].dist) < 0) {
						updated = true
					}
					newEntry := &DistEntryPair{dist: dist, entry: re}
					entryList = append(entryList, newEntry)
					entryMap[str(newEntry.entry.ID)] = newEntry
				}
			}
		}

		sort.Slice(entryList, func(i, j int) bool { return Compare(entryList[i].dist, entryList[j].dist) < 0 })

		if !updated { // no new node found with better distance
			for {
				updated = false
				for k := 0; k < len(entryList) && k < ds.replicationParam; k++ {
					if entryList[k].contacted && !entryList[k].responded {
						entryList = append(entryList[:k], entryList[k+1:]...)
						updated = true
						break
					} else if !entryList[k].contacted {
						entryList[k].contacted = true
						channel := ds.sendAsyncRequest(NewFindNodeRequest(id), entryList[k].entry.Address)
						select {
						case m := <-channel:
							if m != nil {
								entryList[k].responded = true
								for _, re := range m.Message.FindNodeReply.Nodes {
									if _, ok := entryMap[str(re.ID)]; !ok {
										dist := XorDistance(id, re.ID)
										newEntry := &DistEntryPair{dist: dist, entry: re}
										entryList = append(entryList, newEntry)
										entryMap[str(newEntry.entry.ID)] = newEntry
									}
								}
							}
						case <-time.After(time.Duration(int64(ReplyTimeout) * 1000000000)):
						}
						updated = true
						break
					}
				}
				if !updated { // first repicationParam many nodes have replied
					result := make([]*RoutingEntry, 0)
					s := make([]string, 0)
					for k := 0; k < len(entryList) && k < ds.replicationParam; k++ {
						result = append(result, entryList[k].entry)
						s = append(s, fmt.Sprintf("%s [%s]", str(entryList[k].entry.ID), entryList[k].entry.Address))
					}

					if len(s) != 0 {
						//ds.logMessage("Nodes closest to ID %s are {%s}", str(id), strings.Join(s, ", "))
					}
					return result
				}
				sort.Slice(entryList, func(i, j int) bool { return Compare(entryList[i].dist, entryList[j].dist) < 0 })
			}
		}
	}
}

// ValueLookUp finds the value in the distributed storage.
func (ds *DStore) ValueLookUp(key []byte) ([]byte, string) {
	if len(key) < 2+DStoreIDSize {
		return nil, ""
	}

	ds.logMessage("ValueLookUp for key %s", str(key))

	if val, ok := ds.chunkStore.Get(key); ok {
		return val, ds.address
	}

	id := key[2 : 2+DStoreIDSize]

	type DistEntryPair struct {
		dist      []byte
		entry     *RoutingEntry
		contacted bool
		responded bool
	}

	nodes := ds.routingTable.FindKClosest(id, ds.replicationParam)

	entryMap := make(map[string]*DistEntryPair)
	entryList := make([]*DistEntryPair, len(nodes))

	for i, n := range nodes {
		entryList[i] = &DistEntryPair{dist: XorDistance(id, n.ID), entry: n}
		entryMap[str(entryList[i].entry.ID)] = entryList[i]
	}

	sort.Slice(entryList, func(i int, j int) bool { return Compare(entryList[i].dist, entryList[j].dist) < 0 })

	for {
		updated := false
		resChannel := make(chan *MessageAddrPair)
		j := 0
		for i := 0; i < len(entryList) && j < ds.concurrencyParam; i++ { // send to alpha closest nodes
			if entryList[i].contacted {
				continue
			}
			j++
			entryList[i].contacted = true
			channel := ds.sendAsyncRequest(NewFindValueRequest(key), entryList[i].entry.Address)
			go func() {
				select {
				case m := <-channel:
					resChannel <- m
				case <-time.After(time.Duration(ReplyTimeout * 1000000000)):
					resChannel <- nil
				}
			}()
		}

		var foundVal []byte = nil
		var where string = ""

		for k := 0; k < j; k++ {
			msgAddrPair := <-resChannel
			if msgAddrPair == nil {
				continue
			}

			msg, addr := msgAddrPair.Message, msgAddrPair.Address

			temp := entryMap[str(msg.SourceID)]
			if temp == nil {
				continue
			}
			temp.responded = true

			if msg.FindValueReply.Value != nil {
				foundVal = msg.FindValueReply.Value
				where = addr
			}

			if foundVal != nil {
				continue
			}

			for _, re := range msg.FindNodeReply.Nodes {
				if _, ok := entryMap[str(re.ID)]; !ok { // a new node
					dist := XorDistance(id, re.ID)
					if len(entryList) < ds.replicationParam ||
						(len(entryList) >= ds.replicationParam && Compare(dist, entryList[ds.replicationParam-1].dist) < 0) {
						updated = true
					}
					newEntry := &DistEntryPair{dist: dist, entry: re}
					entryList = append(entryList, newEntry)
					entryMap[str(newEntry.entry.ID)] = newEntry
				}
			}
		}

		if foundVal != nil {
			ds.logMessage("Value found for key %s at %s", str(key), where)
			return foundVal, where
		}

		sort.Slice(entryList, func(i, j int) bool { return Compare(entryList[i].dist, entryList[j].dist) < 0 })

		if !updated { // no new node found with better distance
			for {
				updated = false
				for k := 0; k < len(entryList) && k < ds.replicationParam; k++ {
					if entryList[k].contacted && !entryList[k].responded {
						entryList = append(entryList[:k], entryList[k+1:]...)
						updated = true
						break
					} else if !entryList[k].contacted {
						entryList[k].contacted = true
						channel := ds.sendAsyncRequest(NewFindValueRequest(key), entryList[k].entry.Address)
						select {
						case m := <-channel:
							if m != nil {
								if m.Message.FindValueReply.Value != nil {
									ds.logMessage("Value found for key %s at %s", str(key), m.Address)
									return m.Message.FindValueReply.Value, m.Address
								}

								entryList[k].responded = true

								for _, re := range m.Message.FindNodeReply.Nodes {
									if _, ok := entryMap[str(re.ID)]; !ok {
										dist := XorDistance(id, re.ID)
										newEntry := &DistEntryPair{dist: dist, entry: re}
										entryList = append(entryList, newEntry)
										entryMap[str(newEntry.entry.ID)] = newEntry
									}
								}
							}
						case <-time.After(time.Duration(ReplyTimeout * 1000000000)):
						}
						updated = true
						break
					}
				}
				if !updated { // first repicationParam many nodes have replied
					ds.logMessage("Value not found for key %s", str(key))
					return nil, ""
				}
				sort.Slice(entryList, func(i, j int) bool { return Compare(entryList[i].dist, entryList[j].dist) < 0 })
			}
		}
	}
}
