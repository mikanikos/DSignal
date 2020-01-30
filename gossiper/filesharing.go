// Contributors: Andrea Piccione

package gossiper

import (
	"encoding/hex"
	"fmt"
	"time"

	"github.com/mikanikos/DSignal/helpers"
)

func (gossiper *Gossiper) downloadDataFromPeer(fileName, peer string, hash []byte, seqNum uint64) bool {
	// get channel from hashChannel map
	value, _ := gossiper.fileHandler.hashChannels.LoadOrStore(GetKeyFromString(hex.EncodeToString(hash)+peer), make(chan *DataReply))
	replyChan := value.(chan *DataReply)

	// prepare data request
	packet := &GossipPacket{DataRequest: &DataRequest{Origin: gossiper.Name, Destination: peer, HashValue: hash, HopLimit: uint32(hopLimit)}}

	if hw2 {
		printDownloadMessage(fileName, peer, hash, seqNum)
	}

	// send request
	go gossiper.forwardPrivateMessage(packet, &packet.DataRequest.HopLimit, packet.DataRequest.Destination)

	// start timer for repeating request
	requestTimer := time.NewTicker(time.Duration(requestTimeout) * time.Second)
	defer requestTimer.Stop()

	// start timer for stopping download
	stopTimer := time.NewTicker(time.Duration(requestTimeout*10) * time.Second)
	defer stopTimer.Stop()

	for {
		select {
		// incoming reply for this request
		case replyPacket := <-replyChan:

			// save data
			gossiper.fileHandler.hashDataMap.LoadOrStore(hex.EncodeToString(hash), &replyPacket.Data)

			if debug {
				fmt.Println("Got Data")
			}
			return true

		// repeat sending after timeout
		case <-requestTimer.C:
			if hw2 {
				printDownloadMessage(fileName, peer, hash, seqNum)
			}
			go gossiper.forwardPrivateMessage(packet, &packet.DataRequest.HopLimit, packet.DataRequest.Destination)

		// stop after timeout
		case <-stopTimer.C:
			return false
		}
	}
}

// download metafile
func (gossiper *Gossiper) downloadMetafile(fileName, peer string, metaHash []byte) bool {
	return gossiper.downloadDataFromPeer(fileName, peer, metaHash, 0)
}

// request all file chunks of a file
func (gossiper *Gossiper) downloadFileChunks(fileName, destination string, metaHash []byte) {

	// try load data from memory
	metafileStored, mfLoaded := gossiper.fileHandler.hashDataMap.Load(hex.EncodeToString(metaHash))

	// check if I already have metafile information needed for chunks download
	if !mfLoaded {
		if destination == "" {
			if debug {
				fmt.Println("ERROR: file not found in any known peer")
			}
			return
		}

		// download metafile
		if !gossiper.downloadMetafile(fileName, destination, metaHash) {
			if debug {
				fmt.Println("ERROR: the peer doesn't have the metafile or is offline")
			}
			return
		}
		metafileStored, _ = gossiper.fileHandler.hashDataMap.Load(hex.EncodeToString(metaHash))
	}

	// store/get file metadata information
	metafile := *metafileStored.(*[]byte)
	metadataStored, _ := gossiper.fileHandler.filesMetadata.LoadOrStore(GetKeyFromString(hex.EncodeToString(metaHash)+fileName), &FileMetadata{FileName: fileName, MetafileHash: metaHash, ChunkMap: make([]uint64, 0), ChunkCount: uint64(len(metafile) / 32)})
	fileMetadata := metadataStored.(*FileMetadata)
	gossiper.fileHandler.updateChunkMap(fileMetadata, metafileStored.(*[]byte))

	// if already have size, I already have file chunks (maybe with a different name) and there's no need to request it again
	if fileMetadata.Size != 0 {
		if debug {
			fmt.Println("Already have this file")
		}
		if fileMetadata.FileName != fileName {
			if debug {
				fmt.Println("Same file content but different name, just copy it")
			}
			copyFile(fileMetadata.FileName, fileName)
		}
		return
	}

	chunksData := make([]byte, fileMetadata.ChunkCount*fileChunk)
	chunksRetrievedCounter := uint64(0)
	size := int64(0)

	// request each chunk sequentially
	for i := uint64(0); i < fileMetadata.ChunkCount; i++ {
		hashChunk := metafile[i*32 : (i+1)*32]

		// get chunk data from memory if present
		chunkStored, chunkLoaded := gossiper.fileHandler.hashDataMap.Load(hex.EncodeToString(hashChunk))

		// if not present, download it from available peers + destination (if present)
		if !chunkLoaded {
			gossiper.fileHandler.chunkOwnership.Mutex.RLock()
			peersWithChunk, _ := gossiper.fileHandler.chunkOwnership.ChunkOwners[hex.EncodeToString(hashChunk)]
			gossiper.fileHandler.chunkOwnership.Mutex.RUnlock()

			// add destination on top
			if destination != "" {
				peersWithChunk = append([]string{destination}, peersWithChunk...)
			}

			// try to get chunk from every peer
			for _, peer := range peersWithChunk {
				// download chunk
				if gossiper.downloadDataFromPeer(fileName, peer, hashChunk, i+1) {
					if peer == destination {
						gossiper.fileHandler.chunkOwnership.Mutex.Lock()
						gossiper.fileHandler.chunkOwnership.ChunkOwners[hex.EncodeToString(hashChunk)] = helpers.RemoveDuplicatesFromStringSlice(append(gossiper.fileHandler.chunkOwnership.ChunkOwners[hex.EncodeToString(hashChunk)], peer))
						gossiper.fileHandler.chunkOwnership.Mutex.Unlock()
					}
					fileMetadata.ChunkMap = helpers.RemoveDuplicatesFromUint64Slice(helpers.InsertToSortUint64Slice(fileMetadata.ChunkMap, i+1))
					chunkStored, _ = gossiper.fileHandler.hashDataMap.Load(hex.EncodeToString(hashChunk))
					break
				}
			}
		}

		// should be downloaded (either already present or just downloaded)
		if chunkStored != nil {
			chunk := *chunkStored.(*[]byte)
			chunkLen := len(chunk)
			copy(chunksData[i*fileChunk:(int(i)*fileChunk)+chunkLen], chunk)
			size += int64(chunkLen)
			chunksRetrievedCounter++
		}
	}

	if debug {
		fmt.Println("Got " + fmt.Sprint(chunksRetrievedCounter) + " out of " + fmt.Sprint(fileMetadata.ChunkCount))
	}

	// check if I got all chunks
	if fileMetadata.ChunkCount == chunksRetrievedCounter {

		// reconstruct file
		fileReconstructed := make([]byte, size)
		copy(fileReconstructed, chunksData[:size])
		fileMetadata.Size = size

		// save file data to disk
		saveFileOnDisk(fileMetadata.FileName, fileReconstructed)

		if hw2 {
			fmt.Println("RECONSTRUCTED file " + fileName)
		}

		// send it to gui
		go func(f *FileMetadata) {
			gossiper.fileHandler.filesDownloaded <- &FileGUI{Name: f.FileName, MetaHash: hex.EncodeToString(f.MetafileHash), Size: f.Size}
		}(fileMetadata)
	}
}
