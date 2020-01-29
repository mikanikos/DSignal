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
func (client *Client) SendMessage(msg string, dest, file, request *string, keywords string, budget uint64) {

	// create correct packet from arguments
	packet := convertInputToMessage(msg, *dest, *file, *request, keywords, budget)

	if packet != nil {

		// encode
		packetBytes, err := protobuf.Encode(packet)
		helpers.ErrorCheck(err, false)

		// send message to gossiper
		_, err = client.Conn.Write(packetBytes)
		helpers.ErrorCheck(err, false)
	}
}

func getInputType(msg, dest, file, request, keywords string, budget uint64) string {

	if msg != "" && dest == "" && file == "" && request == "" && keywords == "" {
		return "rumor"
	}

	if msg != "" && dest != "" && file == "" && request == "" && keywords == "" {
		return "private"
	}

	if msg == "" && dest == "" && file != "" && request == "" && keywords == "" {
		return "file"
	}

	if msg == "" && file != "" && request != "" && keywords == "" {
		return "request"
	}

	if msg == "" && dest == "" && file == "" && request == "" && keywords != "" {
		return "search"
	}

	return "unknown"
}

// ConvertInputToMessage for client arguments
func convertInputToMessage(msg, dest, file, request, keywords string, budget uint64) *helpers.Message {

	packet := &helpers.Message{}

	// get type of message to create
	switch typeMes := getInputType(msg, dest, file, request, keywords, budget); typeMes {

	case "rumor":
		packet.Text = msg

	case "private":
		packet.Text = msg
		packet.Destination = &dest

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
