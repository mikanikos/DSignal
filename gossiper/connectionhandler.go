package gossiper

import (
	"fmt"
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/DSignal/helpers"
)

// ConnectionHandler struct
type ConnectionHandler struct {
	clientData   *ConnectionData
	GossiperData *ConnectionData
}

// NewConnectionHandler creates new connection handler
func NewConnectionHandler(gossiperAddress, clientAddress string) *ConnectionHandler {

	// connection with other peers
	gossiperData := createConnectionData(gossiperAddress)
	// connection with client
	clientData := createConnectionData(clientAddress)

	return &ConnectionHandler{
		clientData:   clientData,
		GossiperData: gossiperData,
	}
}

// create Connection data
func createConnectionData(addressString string) *ConnectionData {
	// resolve gossiper address
	address, err := net.ResolveUDPAddr("udp4", addressString)
	helpers.ErrorCheck(err, true)

	// get connection for gossiper
	connection, err := net.ListenUDP("udp4", address)
	helpers.ErrorCheck(err, true)

	return &ConnectionData{Address: address, Connection: connection}
}

// ExtendedGossipPacket struct: gossip packet + address of the sender
type ExtendedGossipPacket struct {
	Packet     *GossipPacket
	SenderAddr *net.UDPAddr
}

// process incoming packets from client and send them to the client channel for further processing
func (gossiper *Gossiper) receivePacketsFromClient(clientChannel chan *helpers.Message) {
	for {
		messageFromClient := &helpers.Message{}
		packetBytes := make([]byte, maxBufferSize)

		// read from socket
		n, _, err := gossiper.ConnectionHandler.clientData.Connection.ReadFromUDP(packetBytes)
		helpers.ErrorCheck(err, false)

		if n > maxBufferSize {
			maxBufferSize = maxBufferSize + n
			continue
		}

		// decode message
		err = protobuf.Decode(packetBytes[:n], messageFromClient)
		helpers.ErrorCheck(err, false)

		// send it to channel
		go func(m *helpers.Message) {
			clientChannel <- m
		}(messageFromClient)
	}
}

// process incoming packets from other peers and send them dynamically to the appropriate channel for further processing
func (gossiper *Gossiper) receivePacketsFromPeers() {
	for {
		packetFromPeer := &GossipPacket{}
		packetBytes := make([]byte, maxBufferSize)

		// read from socket
		n, addr, err := gossiper.ConnectionHandler.GossiperData.Connection.ReadFromUDP(packetBytes)
		helpers.ErrorCheck(err, false)

		if n > maxBufferSize {
			maxBufferSize = maxBufferSize + n
			continue
		}

		// add peer
		gossiper.AddPeer(addr)

		// decode message
		err = protobuf.Decode(packetBytes[:n], packetFromPeer)
		helpers.ErrorCheck(err, false)

		// get type of message and send it dynamically to the correct channel
		modeType := getTypeFromGossip(packetFromPeer)

		if modeType != "unknwon" {
			if (modeType == "simple" && simpleMode) || (modeType != "simple" && !simpleMode) {
				packet := &ExtendedGossipPacket{Packet: packetFromPeer, SenderAddr: addr}
				go func(p *ExtendedGossipPacket, m string) {
					PacketChannels[m] <- p
				}(packet, modeType)
			} else {
				fmt.Println("ERROR: message can't be accepted in this operation mode")
			}
		}
	}
}

// send given packet to the address specified
func (connectionHandler *ConnectionHandler) SendPacket(packet *GossipPacket, address *net.UDPAddr) {

	// encode message
	packetToSend, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err, false)

	// send message
	_, err = connectionHandler.GossiperData.Connection.WriteToUDP(packetToSend, address)
	helpers.ErrorCheck(err, false)
}

// broadcast message to all the known peers
func (connectionHandler *ConnectionHandler) BroadcastToPeers(packet *ExtendedGossipPacket, peers []*net.UDPAddr) {
	//peers := gossiper.GetPeersAtomic()
	for _, peer := range peers {
		if peer.String() != packet.SenderAddr.String() {
			connectionHandler.SendPacket(packet.Packet, peer)
		}
	}
}
