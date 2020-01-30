// Contributors: Andrea Piccione, Sergio Roldan, Dewmini Marakkalage

package gossiper

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/mikanikos/DSignal/helpers"
)

// ConnectionData struct: connection + address
type ConnectionData struct {
	Connection *net.UDPConn
	Address    *net.UDPAddr
}

// Gossiper struct
type Gossiper struct {
	// name of the gossiper
	Name string
	// save peers data (neighbours and total number of peers in the network)
	PeersData *PeersData

	// handle connection with peers and client
	ConnectionHandler *ConnectionHandler
	// handle gossip messages and status messages
	gossipHandler *GossipHandler
	// handle routing table and forwarding
	routingHandler *RoutingHandler
	// handle file indexing, sharing and searching
	fileHandler *FileHandler
	// handle abstractions for the blockchain (gossip with confirmation, tlc and qsc)
	blockchainHandler *BlockchainHandler
	// handle storage
	DstorageHandler *DStorageHandler
}

// NewGossiper constructor
func NewGossiper(name, gossiperAddress, clientAddress, peers string, peersNum uint64) *Gossiper {

	// init app
	Init()

	gossiper := &Gossiper{
		Name:      name,
		PeersData: createPeersData(peers, peersNum),

		ConnectionHandler: NewConnectionHandler(gossiperAddress, clientAddress),
		gossipHandler:     NewGossipHandler(),
		routingHandler:    NewRoutingHandler(),
		fileHandler:       NewFileHandler(),
		blockchainHandler: NewBlockchainHandler(),
		DstorageHandler:   NewDStorageHandler(name, gossiperAddress),
	}

	return gossiper

}

// SetAppConstants based on parameters
func SetAppConstants(simple, hw3ex2, hw3ex3, hw3ex4, ackAll, signal bool, hopLimitVal, stubbornTimeoutVal, rtimer, antiEntropy uint) {
	simpleMode = simple
	hw3ex2Mode = hw3ex2
	hw3ex3Mode = hw3ex3
	hw3ex4Mode = hw3ex4
	ackAllMode = ackAll
	SignalMode = signal

	// if qsc, set tlc too
	if hw3ex4Mode {
		hw3ex3Mode = true
	}

	// if tlc, set gossip with confirmation too
	if hw3ex3Mode {
		hw3ex3Mode = true
	}

	hopLimit = int(hopLimitVal)
	stubbornTimeout = int(stubbornTimeoutVal)
	routeRumorTimeout = int(rtimer)
	antiEntropyTimeout = int(antiEntropy)
}

// Init app structures and environments
func Init() {
	// initialize channels used to exchange packets in the app
	initPacketChannels()

	// create working directories for shared and downloaded files
	createWorkingDirectories()
}

// initPacketChannels that are used in the app
func initPacketChannels() {
	// initialize channels used in the application
	PacketChannels = make(map[string]chan *ExtendedGossipPacket)
	for _, t := range modeTypes {
		if (t != "simple" && !simpleMode) || (t == "simple" && simpleMode) {
			PacketChannels[t] = make(chan *ExtendedGossipPacket, maxChannelSize)
		}
	}
}

// createWorkingDirectories for shared and downloaded files at the base directory
func createWorkingDirectories() {
	wd, err := os.Getwd()
	helpers.ErrorCheck(err, true)

	shareFolder = wd + shareFolder
	downloadFolder = wd + downloadFolder

	os.Mkdir(shareFolder, os.ModePerm)
	os.Mkdir(downloadFolder, os.ModePerm)
}

// Run application
func (gossiper *Gossiper) Run(address string) {

	rand.Seed(time.Now().UnixNano())

	// create client channel
	clientChannel := make(chan *helpers.Message, maxChannelSize)
	SignalChannel = make(chan *helpers.Message, maxChannelSize)

	go gossiper.processClientMessages(clientChannel)

	// start processing on separate goroutines

	go gossiper.processSimpleMessages()
	go gossiper.processStatusMessages()
	go gossiper.processRumorMessages()
	go gossiper.startAntiEntropy()

	go gossiper.startRouteRumormongering()
	go gossiper.processPrivateMessages()

	go gossiper.processDataRequest()
	go gossiper.processDataReply()

	go gossiper.processSearchRequest()
	go gossiper.processSearchReply()

	go gossiper.processTLCMessage()
	go gossiper.processTLCAck()
	go gossiper.handleTLCMessage()

	go gossiper.processClientBlocks()

	// listen for incoming packets
	go gossiper.receivePacketsFromClient(clientChannel)
	go gossiper.receivePacketsFromPeers()

	go gossiper.processDStorage()

	gossiper.DstorageHandler.DStore.Init(helpers.GetArrayStringFromAddresses(gossiper.GetPeers()))
	go func() {
		for {
			m := <-gossiper.DstorageHandler.DStore.GetSendChannel()
			
			if m.Address == address {
				continue
			}
			gossiper.ConnectionHandler.SendPacket(&GossipPacket{DStoreMessage: m.Message}, gossiper.GetPeerFromString(m.Address))
		}
	}()

	if debug {
		fmt.Println("Gossiper running")
	}
}

// getters

// GetName of the gossiper
func (gossiper *Gossiper) GetName() string {
	return gossiper.Name
}

// GetRound of the gossiper
func (gossiper *Gossiper) GetRound() uint32 {
	return gossiper.blockchainHandler.myTime
}

// GetSearchedFiles util
func (gossiper *Gossiper) GetSearchedFiles() chan *FileGUI {
	return gossiper.fileHandler.filesSearched
}

// GetIndexedFiles util
func (gossiper *Gossiper) GetIndexedFiles() chan *FileGUI {
	return gossiper.fileHandler.filesIndexed
}

// GetDownloadedFiles util
func (gossiper *Gossiper) GetDownloadedFiles() chan *FileGUI {
	return gossiper.fileHandler.filesDownloaded
}

// GetLatestRumorMessages util
func (gossiper *Gossiper) GetLatestRumorMessages() chan *RumorMessage {
	return gossiper.gossipHandler.latestRumors
}

// GetBlockchainLogs util
func (gossiper *Gossiper) GetBlockchainLogs() chan string {
	return gossiper.blockchainHandler.blockchainLogs
}