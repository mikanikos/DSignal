// Authors: Andrea Piccione

package gossiper

import (
	"encoding/hex"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mikanikos/DSignal/helpers"
)

// SafeRequestMap struct
type SafeRequestMap struct {
	OriginTimeMap map[string]time.Time
	Mutex         sync.RWMutex
}

// handle single search result
func (gossiper *Gossiper) handleSearchResult(origin string, res *SearchResult) {

	value, loaded := gossiper.fileHandler.filesMetadata.LoadOrStore(hex.EncodeToString(res.MetafileHash)+res.FileName, &FileMetadata{FileName: res.FileName, MetafileHash: res.MetafileHash, ChunkCount: res.ChunkCount, ChunkMap: make([]uint64, 0)})
	fileMetadata := value.(*FileMetadata)

	// if not already present, need to save data
	if !loaded {
		printSearchMatchMessage(origin, res)

		// send it to gui
		go func(f *FileMetadata) {
			gossiper.fileHandler.filesSearched <- &FileGUI{Name: f.FileName, MetaHash: hex.EncodeToString(f.MetafileHash), Size: f.Size}
		}(fileMetadata)

		// download metafile and update metadata in background
		go func(fMeta *FileMetadata, o string, chunkMap []uint64) {
			// download metafile, if needed
			metafileStored, mLoaded := gossiper.fileHandler.hashDataMap.Load(hex.EncodeToString(fMeta.MetafileHash))
			if mLoaded || gossiper.downloadMetafile(fMeta.FileName, o, fMeta.MetafileHash) {
				// update chunkmap based on current chunks
				metafileStored, _ = gossiper.fileHandler.hashDataMap.Load(hex.EncodeToString(fMeta.MetafileHash))

				gossiper.fileHandler.updateChunkOwnerMap(o, chunkMap, fileMetadata, metafileStored.(*[]byte))
				gossiper.fileHandler.updateChunkMap(fileMetadata, metafileStored.(*[]byte))
			}
		}(fileMetadata, origin, res.ChunkMap)
	}
}

// check if incoming search request is recent
func (fileHandler *FileHandler) isRecentSearchRequest(searchRequest *SearchRequest) bool {

	// sort and get keywords single string as key
	sort.Strings(searchRequest.Keywords)
	identifier := GetKeyFromString(searchRequest.Origin + strings.Join(searchRequest.Keywords, ""))
	stored := fileHandler.lastSearchRequests.OriginTimeMap[identifier]

	// if not stored or timeout has expired, can take it otherwise no
	if stored.IsZero() || stored.Add(searchRequestDuplicateTimeout).Before(time.Now()) {

		// save new information and reset timer
		if fileHandler.lastSearchRequests.OriginTimeMap[identifier].IsZero() {
			fileHandler.lastSearchRequests.OriginTimeMap[identifier] = time.Now()
		} else {
			last := fileHandler.lastSearchRequests.OriginTimeMap[identifier]
			last = time.Now()
			fileHandler.lastSearchRequests.OriginTimeMap[identifier] = last
		}
		return false
	}
	return true
}

// search file with budget
func (gossiper *Gossiper) searchFilesWithTimeout(extPacket *ExtendedGossipPacket, increment bool) {

	if debug {
		fmt.Println("Got search request")
	}

	keywords := extPacket.Packet.SearchRequest.Keywords
	budget := extPacket.Packet.SearchRequest.Budget

	// decrement budget at source
	extPacket.Packet.SearchRequest.Budget = budget - 1

	if budget <= 0 {
		if debug {
			fmt.Println("Budget too low")
		}
		return
	}

	// forward request
	gossiper.forwardSearchRequestWithBudget(extPacket)

	// start timer
	timer := time.NewTicker(time.Duration(searchTimeout) * time.Second)
	for {
		select {
		case <-timer.C:

			// if budget too high. timeout
			if budget > uint64(maxBudget) {
				timer.Stop()
				if debug {
					fmt.Println("Search timeout")
				}
				return
			}

			// if reached matching threshold, search finished
			if gossiper.fileHandler.isFileMatchThresholdReached(keywords) {
				timer.Stop()
				if hw3 {
					fmt.Println("SEARCH FINISHED")
				}
				return
			}

			// increment budget if it was not specified by argument
			if increment {
				extPacket.Packet.SearchRequest.Budget = budget * 2
				budget = extPacket.Packet.SearchRequest.Budget
			}
			if debug {
				fmt.Println("Sending request")
			}

			// send new request with updated budget
			extPacket.Packet.SearchRequest.Budget = budget - 1
			gossiper.forwardSearchRequestWithBudget(extPacket)
		}
	}
}

// search locally for files matches given keywords
func (fileHandler *FileHandler) isFileMatchThresholdReached(keywords []string) bool {
	matches := make([]string, 0)

	// get over all the files
	fileHandler.filesMetadata.Range(func(key interface{}, value interface{}) bool {
		fileMetadata := value.(*FileMetadata)
		// check if match at least one keyword, if all chunks locations are known and it's not a file I already have
		if containsKeyword(fileMetadata.FileName, keywords) && fileMetadata.Size == 0 && fileHandler.checkAllChunksLocation(fileMetadata) {
			matches = append(matches, fileMetadata.FileName)
		}
		// check if threshold reached and stop iterating in that case
		return !(len(matches) == matchThreshold)
	})

	if debug {
		fmt.Println("Got " + fmt.Sprint(len(matches)))
	}
	return len(matches) == matchThreshold
}

// process search request and search all matches for the keywords given
func (fileHandler *FileHandler) searchMatchingLocalFiles(keywords []string) []*SearchResult {
	//keywords := extPacket.Packet.SearchRequest.Keywords
	searchResults := make([]*SearchResult, 0)

	fileHandler.filesMetadata.Range(func(key interface{}, value interface{}) bool {
		fileMetadata := value.(*FileMetadata)
		// check if match at least one keyword
		if containsKeyword(fileMetadata.FileName, keywords) && len(fileMetadata.ChunkMap) != 0 {
			searchResults = append(searchResults, &SearchResult{FileName: fileMetadata.FileName, MetafileHash: fileMetadata.MetafileHash, ChunkCount: fileMetadata.ChunkCount, ChunkMap: fileMetadata.ChunkMap})
		}
		return true
	})
	return searchResults
}

// forward request as evenly as possible to the known peers
func (gossiper *Gossiper) forwardSearchRequestWithBudget(extPacket *ExtendedGossipPacket) {

	peers := gossiper.GetPeers()
	peersChosen := []*net.UDPAddr{extPacket.SenderAddr}
	availablePeers := helpers.DifferenceString(peers, peersChosen)

	// if budget is valid and I know some other peers (don't sending back to the peers I received it)
	if extPacket.Packet.SearchRequest.Budget > 0 && len(availablePeers) != 0 {

		if debug {
			fmt.Println("Handling request from " + extPacket.SenderAddr.String() + " with total budget " + fmt.Sprint(extPacket.Packet.SearchRequest.Budget))
		}

		// compute minimal budget for each peer and budget to share among all the peers
		budgetForEach := extPacket.Packet.SearchRequest.Budget / uint64(len(availablePeers))
		budgetToShare := extPacket.Packet.SearchRequest.Budget % uint64(len(availablePeers))

		// repetedly get random peer, send minimal budget + 1 if some budget to share left
		for len(availablePeers) != 0 {

			if budgetToShare == 0 && budgetForEach == 0 {
				return
			}

			randomPeer := getRandomPeer(availablePeers)
			if budgetToShare > 0 {
				extPacket.Packet.SearchRequest.Budget = budgetForEach + 1
				budgetToShare = budgetToShare - 1
			} else {
				extPacket.Packet.SearchRequest.Budget = budgetForEach
			}

			if debug {
				fmt.Println("Sending request to " + randomPeer.String() + " with budget " + fmt.Sprint(extPacket.Packet.SearchRequest.Budget))
			}

			// send packet and remove current peer
			gossiper.ConnectionHandler.SendPacket(extPacket.Packet, randomPeer)
			peersChosen = append(peersChosen, randomPeer)
			availablePeers = helpers.DifferenceString(gossiper.GetPeers(), peersChosen)
		}
	}
}
