package gossiper

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/mikanikos/DSignal/helpers"
	"github.com/mikanikos/DSignal/storage"
)

// FOUND IT MORE USEFUL TO PUT ALL MAIN PROCESSING GOROUTINES IN ONE PLACE

// process tlc message
func (gossiper *Gossiper) processTLCMessage() {
	for extPacket := range PacketChannels["tlcMes"] {

		// handle gossip message
		go gossiper.HandleGossipMessage(extPacket, extPacket.Packet.TLCMessage.Origin, extPacket.Packet.TLCMessage.ID)

		// handle tlc message
		if (hw3ex2Mode || hw3ex3Mode || hw3ex4Mode) && extPacket.Packet.TLCMessage.Origin != gossiper.Name {
			go func(e *ExtendedGossipPacket) {
				PacketChannels["tlcCausal"] <- e
			}(extPacket)
		}
	}
}

// process tlc acks
func (gossiper *Gossiper) processTLCAck() {
	for extPacket := range PacketChannels["tlcAck"] {
		if extPacket.Packet.Ack.Destination == gossiper.Name {

			if debug {
				fmt.Println("Got ack for " + fmt.Sprint(extPacket.Packet.Ack.ID) + " from " + extPacket.Packet.Ack.Origin)
			}

			// send to the ack channel
			go func(v *TLCAck) {
				gossiper.blockchainHandler.tlcAckChan <- v
			}(extPacket.Packet.Ack)
		} else {
			// forward tlc ack to peers
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.Ack.HopLimit, extPacket.Packet.Ack.Destination)
		}
	}
}

// process search request
func (gossiper *Gossiper) processSearchRequest() {
	for extPacket := range PacketChannels["searchRequest"] {
		if !gossiper.fileHandler.isRecentSearchRequest(extPacket.Packet.SearchRequest) {
			// check if search request is not recent
			if extPacket.Packet.SearchRequest.Origin != gossiper.Name {

				// send matching local files
				searchResults := gossiper.fileHandler.searchMatchingLocalFiles(extPacket.Packet.SearchRequest.Keywords)
				if len(searchResults) != 0 {
					if debug {
						fmt.Println("Sending search results")
					}
					searchReply := &SearchReply{Origin: gossiper.Name, Destination: extPacket.Packet.SearchRequest.Origin, HopLimit: uint32(hopLimit), Results: searchResults}
					packetToSend := &GossipPacket{SearchReply: searchReply}

					go gossiper.forwardPrivateMessage(packetToSend, &packetToSend.SearchReply.HopLimit, packetToSend.SearchReply.Destination)
				}
			} else {
				if debug {
					fmt.Println("Too recent request!!!!")
				}
			}

			// forward request with left budget
			extPacket.Packet.SearchRequest.Budget = extPacket.Packet.SearchRequest.Budget - 1
			go gossiper.forwardSearchRequestWithBudget(extPacket)
		}
	}
}

// process search replies
func (gossiper *Gossiper) processSearchReply() {
	for extPacket := range PacketChannels["searchReply"] {

		// if it's for me, I handle search results
		if extPacket.Packet.SearchReply.Destination == gossiper.Name {

			searchResults := extPacket.Packet.SearchReply.Results
			for _, res := range searchResults {
				go gossiper.handleSearchResult(extPacket.Packet.SearchReply.Origin, res)
			}
		} else {
			// if not for me, I forward the reply
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.SearchReply.HopLimit, extPacket.Packet.SearchReply.Destination)
		}
	}
}

func (gossiper *Gossiper) processDStorage() {
	for extPacket := range PacketChannels["dstorageMessage"] {
		go func() {
			gossiper.DstorageHandler.DStore.GetReceiveChannel() <- &storage.MessageAddrPair{Message: extPacket.Packet.DStoreMessage, Address: extPacket.SenderAddr.String()}
		}()
	}
}

// process data request
func (gossiper *Gossiper) processDataRequest() {
	for extPacket := range PacketChannels["dataRequest"] {

		// if for me, I I handle the data request
		if extPacket.Packet.DataRequest.Destination == gossiper.Name {

			if debug {
				fmt.Println("Got data request")
			}

			dataReply := &DataReply{Origin: gossiper.Name, Destination: extPacket.Packet.DataRequest.Origin, HopLimit: uint32(hopLimit), HashValue: extPacket.Packet.DataRequest.HashValue}
			packetToSend := &GossipPacket{DataReply: dataReply}

			// try checking hash from stored data
			value, loaded := gossiper.fileHandler.hashDataMap.Load(hex.EncodeToString(extPacket.Packet.DataRequest.HashValue))

			if loaded {
				dataRequested := value.(*[]byte)
				packetToSend.DataReply.Data = *dataRequested

				if debug {
					fmt.Println("Sent data requested")
				}
			}

			// send data found (or nil) to peer who requested
			go gossiper.forwardPrivateMessage(packetToSend, &packetToSend.DataReply.HopLimit, packetToSend.DataReply.Destination)

		} else {
			// if not for me, I forward the request
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.DataRequest.HopLimit, extPacket.Packet.DataRequest.Destination)
		}
	}
}

// process data reply
func (gossiper *Gossiper) processDataReply() {
	for extPacket := range PacketChannels["dataReply"] {

		if debug {
			fmt.Println("Got data reply from " + extPacket.Packet.DataReply.Destination)
		}

		// if for me, handle data reply
		if extPacket.Packet.DataReply.Destination == gossiper.Name {

			if debug {
				fmt.Println("For me")
			}

			// check integrity of the hash
			if extPacket.Packet.DataReply.Data != nil && checkHash(extPacket.Packet.DataReply.HashValue, extPacket.Packet.DataReply.Data) {

				if debug {
					fmt.Println("DATA VALID")
				}

				value, _ := gossiper.fileHandler.hashChannels.Load(GetKeyFromString(hex.EncodeToString(extPacket.Packet.DataReply.HashValue) + extPacket.Packet.DataReply.Origin))

				// send it to the appropriate channel
				channel := value.(chan *DataReply)
				go func(c chan *DataReply, d *DataReply) {
					c <- d
				}(channel, extPacket.Packet.DataReply)
			}

		} else {
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.DataReply.HopLimit, extPacket.Packet.DataReply.Destination)
		}
	}
}

// process private message
func (gossiper *Gossiper) processPrivateMessages() {
	for extPacket := range PacketChannels["private"] {

		// update routing table
		gossiper.routingHandler.updateRoutingTable(extPacket.Packet.Private.Destination, extPacket.Packet.Private.Text, extPacket.Packet.Private.ID, extPacket.SenderAddr)

		// if for me, handle private message
		if extPacket.Packet.Private.Destination == gossiper.Name {
			if hw2 {
				gossiper.printPeerMessage(extPacket, gossiper.GetPeers())
			}
			// send it to gui
			go func(p *PrivateMessage) {
				gossiper.gossipHandler.latestRumors <- &RumorMessage{Text: p.Text, Origin: p.Origin}
			}(extPacket.Packet.Private)

		} else {
			// if not for me, forward message
			go gossiper.forwardPrivateMessage(extPacket.Packet, &extPacket.Packet.Private.HopLimit, extPacket.Packet.Private.Destination)
		}
	}
}

// process client messages
func (gossiper *Gossiper) processClientMessages(clientChannel chan *helpers.Message) {
	for message := range clientChannel {

		packet := &ExtendedGossipPacket{SenderAddr: gossiper.ConnectionHandler.GossiperData.Address}

		// get type of the message
		switch typeMessage := getTypeFromMessage(message); typeMessage {

		case "simple":
			if hw1 {
				printClientMessage(message, gossiper.GetPeers())
			}

			// create simple packet and broadcast
			simplePacket := &SimpleMessage{Contents: message.Text, OriginalName: gossiper.Name, RelayPeerAddr: gossiper.ConnectionHandler.GossiperData.Address.String()}
			packet.Packet = &GossipPacket{Simple: simplePacket}

			go gossiper.ConnectionHandler.BroadcastToPeers(packet, gossiper.GetPeers())

		case "ratchet":
			if SignalMode {
				
				go func(m *helpers.Message) {
					SignalChannel <- m
				}(message)

				continue
			}

		case "private":
			if hw2 {
				printClientMessage(message, gossiper.GetPeers())
			}

			if SignalMode {

				go func(m *helpers.Message) {
					SignalChannel <- m
				}(message)

				continue
			}

			// create private message and forward it
			privatePacket := &PrivateMessage{Origin: gossiper.Name, ID: 0, Text: message.Text, Destination: *message.Destination, HopLimit: uint32(hopLimit)}
			packet.Packet = &GossipPacket{Private: privatePacket}

			go func(p *PrivateMessage) {
				gossiper.gossipHandler.latestRumors <- &RumorMessage{Text: p.Text, Origin: p.Origin}
			}(privatePacket)

			go gossiper.forwardPrivateMessage(packet.Packet, &packet.Packet.Private.HopLimit, packet.Packet.Private.Destination)

		case "rumor":
			printClientMessage(message, gossiper.GetPeers())

			// create rumor
			extPacket := gossiper.CreateRumorMessage(message.Text)

			// rumor monger it
			go gossiper.StartRumorMongering(extPacket, gossiper.Name, extPacket.Packet.Rumor.ID)

		case "file":
			gossiper.indexFile(message.File)

		case "dataRequest":
			go gossiper.downloadFileChunks(*message.File, *message.Destination, *message.Request)

		case "searchRequest":

			// create search request packet and handle it
			keywordsSplitted := helpers.RemoveDuplicatesFromStringSlice(strings.Split(*message.Keywords, ","))

			requestPacket := &SearchRequest{Origin: gossiper.Name, Keywords: keywordsSplitted, Budget: *message.Budget}
			packet.Packet = &GossipPacket{SearchRequest: requestPacket}

			// if 0, means bufget was not specified: so use default budget and increment after timeout
			needIncrement := (*message.Budget == 0)

			if needIncrement {
				requestPacket.Budget = uint64(defaultBudget)
			}

			go gossiper.searchFilesWithTimeout(packet, needIncrement)

		default:
			if debug {
				fmt.Println("Unkown packet!")
			}
		}
	}
}

// process simple message
func (gossiper *Gossiper) processSimpleMessages() {
	for extPacket := range PacketChannels["simple"] {

		if hw1 {
			gossiper.printPeerMessage(extPacket, gossiper.GetPeers())
		}

		// set relay address and broadcast
		extPacket.Packet.Simple.RelayPeerAddr = gossiper.ConnectionHandler.GossiperData.Address.String()

		go gossiper.ConnectionHandler.BroadcastToPeers(extPacket, gossiper.GetPeers())
	}
}

// process rumor message
func (gossiper *Gossiper) processRumorMessages() {
	for extPacket := range PacketChannels["rumor"] {

		// handle gossip message
		gossiper.HandleGossipMessage(extPacket, extPacket.Packet.Rumor.Origin, extPacket.Packet.Rumor.ID)
	}
}

// process status message
func (gossiper *Gossiper) processStatusMessages() {
	for extPacket := range PacketChannels["status"] {

		if hw1 {
			printStatusMessage(extPacket, gossiper.GetPeers())
		}

		// get status channel for peer and send it there
		value, exists := gossiper.gossipHandler.statusChannels.LoadOrStore(extPacket.SenderAddr.String(), make(chan *ExtendedGossipPacket, maxChannelSize))
		channelPeer := value.(chan *ExtendedGossipPacket)
		if !exists {
			go gossiper.handlePeerStatus(channelPeer)
		}
		go func(c chan *ExtendedGossipPacket, p *ExtendedGossipPacket) {
			c <- p
		}(channelPeer, extPacket)
	}
}
