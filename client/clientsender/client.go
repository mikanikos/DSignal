package clientsender

import (
	"encoding/hex"
	"fmt"
	"net"

	"github.com/dedis/protobuf"
	"github.com/mikanikos/DSignal/helpers"
)

// Client struct
type Client struct {
	GossiperAddr *net.UDPAddr
	Conn         *net.UDPConn
}

// NewClient init
func NewClient(uiPort string) *Client {
	// resolve gossiper address
	gossiperAddr, err := net.ResolveUDPAddr("udp4", helpers.BaseAddress+":"+uiPort)
	helpers.ErrorCheck(err, true)
	// establish connection
	conn, err := net.DialUDP("udp4", nil, gossiperAddr)
	helpers.ErrorCheck(err, true)

	return &Client{
		GossiperAddr: gossiperAddr,
		Conn:         conn,
	}
}

// SendMessage to gossiper
func (client *Client) SendMessage(msg string, dest, file, request *string, keywords string, budget uint64, identity *string) {

	// create correct packet from arguments
	packet := convertInputToMessage(msg, *dest, *file, *request, keywords, budget, *identity)

	if packet != nil {

		// encode
		packetBytes, err := protobuf.Encode(packet)
		helpers.ErrorCheck(err, false)

		// send message to gossiper
		_, err = client.Conn.Write(packetBytes)
		helpers.ErrorCheck(err, false)
	}
}

func getInputType(msg, dest, file, request, keywords string, budget uint64, identity string) string {

	if msg != "" && dest == "" && file == "" && request == "" && keywords == "" && identity == "" {
		return "rumor"
	}

	if msg != "" && dest != "" && file == "" && request == "" && keywords == "" && identity == "" {
		return "private"
	}

	if msg != "" && dest != "" && file == "" && request == "" && keywords == "" && identity != "" {
		return "ratchet"
	}

	if msg == "" && dest == "" && file != "" && request == "" && keywords == "" && identity == "" {
		return "file"
	}

	if msg == "" && file != "" && request != "" && keywords == "" && identity == "" {
		return "request"
	}

	if msg == "" && dest == "" && file == "" && request == "" && keywords != "" && identity == "" {
		return "search"
	}

	return "unknown"
}

// ConvertInputToMessage for client arguments
func convertInputToMessage(msg, dest, file, request, keywords string, budget uint64, identity string) *helpers.Message {

	packet := &helpers.Message{}

	// get type of message to create
	switch typeMes := getInputType(msg, dest, file, request, keywords, budget, identity); typeMes {

	case "rumor":
		packet.Text = msg

	case "private":
		packet.Text = msg
		packet.Destination = &dest

	case "ratchet":
		packet.Text = msg
		packet.Destination = &dest
		packet.Identity = &identity

	case "file":
		packet.File = &file

	case "request":
		decodeRequest, err := hex.DecodeString(request)
		if err != nil {
			fmt.Println("ERROR (Unable to decode hex hash)")
			return nil
		}
		packet.Request = &decodeRequest
		packet.File = &file
		packet.Destination = &dest

	case "search":
		packet.Keywords = &keywords
		packet.Budget = &budget

	default:
		fmt.Println("ERROR (Bad argument combination)")
		return nil
	}

	return packet
}
