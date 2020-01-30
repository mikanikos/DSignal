// Contributors: Andrea Piccione

package gossiper

import (
	"encoding/hex"
)

// functions for UI, quite self-explanatory

// FileGUI struct
type FileGUI struct {
	Name     string
	MetaHash string
	Size     int64
}

// GetMessagesList for GUI
func GetMessagesList(channel chan *RumorMessage) []RumorMessage {

	bufferLength := len(channel)
	messages := make([]RumorMessage, bufferLength)
	for i := 0; i < bufferLength; i++ {
		message := <-channel
		messages[i] = *message
	}

	return messages
}

// GetFilesList for GUI
func GetFilesList(channel chan *FileGUI) []FileGUI {

	bufferLength := len(channel)
	files := make([]FileGUI, bufferLength)
	for i := 0; i < bufferLength; i++ {
		file := <-channel
		files[i] = *file
	}
	return files
}

// GetBlockchainList for GUI
func GetBlockchainList(channel chan string) []string {

	bufferLength := len(channel)
	logs := make([]string, bufferLength)
	for i := 0; i < bufferLength; i++ {
		log := <-channel
		logs[i] = log
	}
	return logs
}

// GetBlockchain for GUI
func (gossiper *Gossiper) GetBlockchain() []FileGUI {

	filesConsensus := make([]FileGUI, 0)
	blockHash := gossiper.blockchainHandler.topBlockchainHash
	for blockHash != [32]byte{} {
		value, _ := gossiper.blockchainHandler.committedHistory.Load(gossiper.blockchainHandler.topBlockchainHash)
		block := value.(BlockPublish)
		filesConsensus = append(filesConsensus, FileGUI{Name: block.Transaction.Name, MetaHash: hex.EncodeToString(block.Transaction.MetafileHash), Size: block.Transaction.Size})
		blockHash = block.PrevHash
	}
	return filesConsensus
}
