package gossiper

import (
	"encoding/hex"
	"fmt"
	"net"
	"strings"

	"github.com/mikanikos/DSignal/helpers"
)

// printing functions, quite self-explanatory

func printStatusMessage(extPacket *ExtendedGossipPacket, peers []*net.UDPAddr) {
	message := "STATUS from " + extPacket.SenderAddr.String() + " "
	for _, value := range extPacket.Packet.Status.Want {
		message = message + "peer " + value.Identifier + " nextID " + fmt.Sprint(value.NextID) + " "
	}
	if hw1 {
		fmt.Println(message[:len(message)-1])
		printPeers(peers)
	}
}

func printSearchMatchMessage(origin string, res *SearchResult) {
	message := "FOUND match " + res.FileName + " at " + origin + " metafile=" + hex.EncodeToString(res.MetafileHash) + " chunks="
	for _, elem := range res.ChunkMap {
		message = message + fmt.Sprint(elem) + ","
	}
	if hw3 {
		fmt.Println(message[:len(message)-1])
	}
}

func (gossiper *Gossiper) printPeerMessage(extPacket *ExtendedGossipPacket, peers []*net.UDPAddr) {

	switch typePacket := getTypeFromGossip(extPacket.Packet); typePacket {

	case "simple":
		if hw1 {
			fmt.Println("SIMPLE MESSAGE origin " + extPacket.Packet.Simple.OriginalName + " from " + extPacket.Packet.Simple.RelayPeerAddr + " contents " + extPacket.Packet.Simple.Contents)
		}
	case "private":
		if hw2 {
			fmt.Println("PRIVATE origin " + extPacket.Packet.Private.Origin + " hop-limit " + fmt.Sprint(extPacket.Packet.Private.HopLimit) + " contents " + extPacket.Packet.Private.Text)
		}
	case "rumor":
		//fmt.Println("RUMOR origin " + extPacket.Packet.Rumor.Origin + " from " + extPacket.SenderAddr.String() + " ID " + fmt.Sprint(extPacket.Packet.Rumor.ID) + " contents " + extPacket.Packet.Rumor.Text)

	case "tlcMes":
		messageToPrint := "GOSSIP origin " + extPacket.Packet.TLCMessage.Origin +
			" ID " + fmt.Sprint(extPacket.Packet.TLCMessage.ID) +
			" filename " + extPacket.Packet.TLCMessage.TxBlock.Transaction.Name +
			" size " + fmt.Sprint(extPacket.Packet.TLCMessage.TxBlock.Transaction.Size) +
			" metahash " + hex.EncodeToString(extPacket.Packet.TLCMessage.TxBlock.Transaction.MetafileHash)

		if extPacket.Packet.TLCMessage.Confirmed > -1 {
			fmt.Println("CONFIRMED " + messageToPrint)

			message := "CONFIRMED " + messageToPrint
			go func(m string) {
				gossiper.blockchainHandler.blockchainLogs <- message
			}(message)

		} else {
			fmt.Println("UNCONFIRMED " + messageToPrint)
		}
	}

	if hw1 {
		printPeers(peers)
	}
}

func printClientMessage(message *helpers.Message, peers []*net.UDPAddr) {
	if message.Destination != nil {
		fmt.Println("CLIENT MESSAGE " + message.Text + " dest " + *message.Destination)
	} else {
		fmt.Println("CLIENT MESSAGE " + message.Text)
	}
	if hw1 {
		printPeers(peers)
	}
}

func printPeers(peers []*net.UDPAddr) {
	listPeers := helpers.GetArrayStringFromAddresses(peers)
	if hw1 {
		fmt.Println("PEERS " + strings.Join(listPeers, ","))
	}
}

func printDownloadMessage(fileName, destination string, hash []byte, seqNum uint64) {
	// metafile
	if seqNum == 0 {
		fmt.Println("DOWNLOADING metafile of " + fileName + " from " + destination)
	} else {
		// chunk
		fmt.Println("DOWNLOADING " + fileName + " chunk " + fmt.Sprint(seqNum) + " from " + destination)
	}
}

func (gossiper *Gossiper) printRoundMessage(round uint32, confirmations map[string]*TLCMessage) {
	message := "ADVANCING TO round " + fmt.Sprint(round) + " BASED ON CONFIRMED MESSAGES "

	i := 1
	for key, value := range confirmations {
		message = message + "origin" + fmt.Sprint(i) + " " + key + " ID" + fmt.Sprint(i) + " " + fmt.Sprint(value.ID) + ", "
		i++
	}
	if hw3ex3Mode {
		fmt.Println(message[:len(message)-2])
		go func(m string) {
			gossiper.blockchainHandler.blockchainLogs <- m[:len(m)-2]
		}(message)
	}
}

func printConfirmMessage(id uint32, witnesses map[string]uint32) {
	message := "RE-BROADCAST ID " + fmt.Sprint(id) + " WITNESSES "

	for key := range witnesses {
		message = message + key + ", "
	}
	if hw3ex2Mode {
		fmt.Println(message[:len(message)-2])
	}
}

func (gossiper *Gossiper) printConsensusMessage(tlcChosen *TLCMessage) {

	message := "CONSENSUS ON QSC round " + fmt.Sprint(gossiper.blockchainHandler.myTime) + " message origin " + tlcChosen.Origin + " ID " + fmt.Sprint(tlcChosen.ID) + " filenames " //+ <name_oldest> ... <name_newest>​ size <size> metahash <metahash>"

	filenames := ""

	blockHash := gossiper.blockchainHandler.topBlockchainHash
	for blockHash != [32]byte{} {
		value, _ := gossiper.blockchainHandler.committedHistory.Load(blockHash)
		block := value.(BlockPublish)
		filenames = block.Transaction.Name + " " + filenames
		blockHash = block.PrevHash
	}

	message = message + filenames + "size " + fmt.Sprint(tlcChosen.TxBlock.Transaction.Size) + " metahash " + hex.EncodeToString(tlcChosen.TxBlock.Transaction.MetafileHash)

	fmt.Println(message)
	go func(m string) {
		gossiper.blockchainHandler.blockchainLogs <- m
	}(message)
}
