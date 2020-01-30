// Contributors: Andrea Piccione

package gossiper

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/mikanikos/DSignal/helpers"
)

// some utils used throughout the app, should be easy to understand

// get type of gossip packet based on fields
func getTypeFromGossip(packet *GossipPacket) string {

	// as stated in the handout, we assume every packet has only one field which is not null

	if packet.Simple != nil {
		return "simple"
	} else if packet.Rumor != nil {
		return "rumor"
	} else if packet.Private != nil {
		return "private"
	} else if packet.Status != nil {
		return "status"
	} else if packet.DataRequest != nil {
		return "dataRequest"
	} else if packet.DataReply != nil {
		return "dataReply"
	} else if packet.SearchRequest != nil {
		return "searchRequest"
	} else if packet.SearchReply != nil {
		return "searchReply"
	} else if packet.TLCMessage != nil {
		return "tlcMes"
	} else if packet.Ack != nil {
		return "tlcAck"
	} else if packet.WhisperPacket != nil {
		return "whisperPacket"
	} else if packet.WhisperStatus != nil {
		return "whisperStatus"
	} else if packet.DStoreMessage != nil {
		return "dstorageMessage"
	}

	return "unknown"
}

// get type of client message based on fields
func getTypeFromMessage(message *helpers.Message) string {
	if simpleMode {
		return "simple"
	}

	if message.Destination != nil && message.Text != "" && message.File == nil && message.Request == nil && message.Keywords == nil {
		return "private"
	}

	if message.Text == "" && message.File != nil && message.Request != nil && message.Keywords == nil {
		return "dataRequest"
	}

	if message.Destination == nil && message.Text == "" && message.File != nil && message.Request == nil && message.Keywords == nil {
		return "file"
	}

	if message.Destination == nil && message.Text == "" && message.File == nil && message.Request == nil && message.Keywords != nil {
		return "searchRequest"
	}

	if message.Text != "" && message.Destination == nil && message.File == nil && message.Request == nil && message.Keywords == nil {
		return "rumor"
	}

	return "unknown"
}

// get random peer from list
func getRandomPeer(availablePeers []*net.UDPAddr) *net.UDPAddr {
	indexPeer := rand.Intn(len(availablePeers))
	return availablePeers[indexPeer]
}

// check hash validity
func checkHash(hash []byte, data []byte) bool {
	var hash32 [32]byte
	copy(hash32[:], hash)
	value := sha256.Sum256(data)
	return hash32 == value
}

// copy file on disk
func copyFile(source, target string) {

	source = downloadFolder + source
	target = downloadFolder + target

	sourceFile, err := os.Open(source)
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}
	defer sourceFile.Close()

	targetFile, err := os.Create(target)
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}
	defer targetFile.Close()

	_, err = io.Copy(targetFile, sourceFile)
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}

	err = targetFile.Close()
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}
}

// save file on disk
func saveFileOnDisk(fileName string, data []byte) {
	file, err := os.Create(downloadFolder + fileName)
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.Write(data)
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}

	err = file.Sync()
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}
}

// get list of ids instaed of messages
func getIDForConfirmations(confirmations map[string]*TLCMessage) map[string]uint32 {
	originIDMap := make(map[string]uint32)
	for key, value := range confirmations {
		originIDMap[key] = value.ID
	}
	return originIDMap
}

// check if contains keywords
func containsKeyword(fileName string, keywords []string) bool {
	for _, keyword := range keywords {
		if matched, _ := regexp.MatchString(keyword, fileName); matched || strings.Contains(fileName, keyword) {
			return true
		}
	}
	return false
}

// string -> hashed key
func GetKeyFromString(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// get tlc with highest fitness value
func getTLCWithHighestFitness(confirmations map[string]*TLCMessage) *TLCMessage {

	maxBlock := &TLCMessage{Fitness: 0}
	for _, value := range confirmations {

		if debug {
			fmt.Println(value.Origin + " " + fmt.Sprint(value.ID) + " " + fmt.Sprint(value.Fitness))
		}

		if value.Fitness > maxBlock.Fitness {
			maxBlock = value
		}
	}
	return maxBlock
}

// GetHash function of BlockPublish
func (b *BlockPublish) Hash() (out [32]byte) {
	h := sha256.New()
	h.Write(b.PrevHash[:])
	th := b.Transaction.Hash()
	h.Write(th[:])
	copy(out[:], h.Sum(nil))
	return
}

// GetHash function of TXPublish
func (t *TxPublish) Hash() (out [32]byte) {
	h := sha256.New()
	binary.Write(h, binary.LittleEndian,
		uint32(len(t.Name)))
	h.Write([]byte(t.Name))
	h.Write(t.MetafileHash)
	copy(out[:], h.Sum(nil))
	return
}
