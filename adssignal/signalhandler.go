package adssignal

import (
	"encoding/json"
	"fmt"
	"github.com/dedis/protobuf"
	"github.com/mikanikos/DSignal/gossiper"
	"github.com/mikanikos/DSignal/storage"
	"github.com/mikanikos/DSignal/whisper"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mikanikos/DSignal/helpers"
)

const (
	identityFile       = "Identity.txt"
	identityTimeout    = 1
	hopLimit           = 10
	maxChannelSize     = 1024
	defaultTTL         = 300
	defaultPoWTime     = 1
	defaultPoWRequired = 0.2
)

const (
	identityDH = iota
	messageDH
	ratchet
)

// SignalPacket struct
type SignalPacket struct {
	X3DHIdentity   *X3DHIdentity
	X3DHMessage    *X3DHMessage
	RatchetMessage *RatchetMessage
}

var SignalPacketChannels map[int]chan *SignalPacket

// Signal structure including self identity, state table, identity channels and a mutex
type SignalHandler struct {
	w                *whisper.Whisper
	g                *gossiper.Gossiper
	ds               *storage.DStore
	selfIdentity     SelfIdentity
	stateTable       map[string]*DRatchetState
	identityChannels sync.Map

	mutex sync.RWMutex
}

// Create a new handler
func NewSignalHandler(name string, g *gossiper.Gossiper, w *whisper.Whisper, ds *storage.DStore) *SignalHandler {
	identity := retrieveIdentity(name)
	states := make(map[string]*DRatchetState)

	if identity == nil {
		identity = GenerateIdentity()
		storeIdentity(*identity, name)
	} else {
		retrieveRatchets(name, states)
	}

	return &SignalHandler{
		g: g,
		w: w,
		ds: ds,
		selfIdentity:     *identity,
		stateTable:       states,
		identityChannels: sync.Map{},
	}
}

func (signal *SignalHandler) Run() {
	go signal.processIdentityMessages()
	go signal.processDHMessages()
	go signal.processRatchetMessages()

	for i := 0; i < 3; i++ {
		SignalPacketChannels[i] = make(chan *SignalPacket, maxChannelSize)
	}

	topic := []byte(signal.g.Name)
	topics := make([]whisper.Topic, 0)
	topics = append(topics, whisper.ConvertBytesToTopic(topic))
	crit := whisper.FilterOptions{
		MinPow: defaultPoWRequired,
		Topics: topics,
	}

	filterHash, err := signal.w.NewMessageFilter(crit)
	if err != nil {
		fmt.Printf("failed when creating new filter: %s", err)
	}

	go signal.listenForIncomingMessages(filterHash)

}

func (signal *SignalHandler) listenForIncomingMessages(filterHash string) {

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mess, err := signal.w.GetFilterMessages(filterHash)
			if err != nil {
				fmt.Printf("failed retrieving messages: %s", err)
				continue
			}

			if len(mess) != 0 {
				for _, m := range mess {

					packet := &SignalPacket{}

					// decode message
					err = protobuf.Decode(m.Payload, packet)
					helpers.ErrorCheck(err, false)

					modeType := -1
					if packet.X3DHIdentity != nil {
						modeType = identityDH
					} else if packet.X3DHMessage != nil {
						modeType = messageDH
					} else if packet.RatchetMessage != nil {
						modeType = ratchet
					} else {
						continue
					}

					go func(p *SignalPacket, m int) {
						SignalPacketChannels[m] <- p
					}(packet, modeType)

				}
			}
		}
	}

}

func (signal *SignalHandler) handleClientPrivateMessage() {
	for message := range gossiper.SignalChannel {
		go signal.sendPrivateRatchet(message.Text, *message.Destination)
	}
}

func (signal *SignalHandler) sendWhisperMessage(packet *SignalPacket, hop uint32, destination string) {
	topic := []byte(destination)
	packetToSend, err := protobuf.Encode(packet)
	helpers.ErrorCheck(err, false)
	newMessage := whisper.NewMessage{
		//SymKeyID: symKeyID2,
		TTL:     defaultTTL,
		Topic:   whisper.ConvertBytesToTopic(topic),
		Payload: packetToSend,
		PowTime: defaultPoWTime,
	}

	_, err = signal.w.NewWhisperMessage(newMessage)
	if err != nil {
		fmt.Printf("failed when sending new message: %s", err)
	}
}

// process identity message
func (signal *SignalHandler) processIdentityMessages() {
	for packet := range SignalPacketChannels[identityDH] {

		//signal.g.routingHandler.updateRoutingTable(extPacket.Packet.X3DHIdentity.Destination, "", 0, extPacket.SenderAddr)

		if packet.X3DHIdentity.Destination == signal.g.Name {
			go signal.rStartX3DH(packet.X3DHIdentity)
		} else {
			go signal.sendWhisperMessage(packet, defaultTTL, packet.X3DHIdentity.Destination)
		}
	}
}

// process ratchet message
func (signal *SignalHandler) processRatchetMessages() {
	for packet := range SignalPacketChannels[ratchet] {

		//gossiper.routingHandler.updateRoutingTable(extPacket.Packet.RatchetMessage.Header.Destination, "", uint32(extPacket.Packet.RatchetMessage.Header.N), extPacket.SenderAddr)

		if packet.RatchetMessage.Header.Destination == signal.g.Name {
			msg := signal.receivePrivateRatchet(*packet.RatchetMessage)

			if msg != nil {
				fmt.Println("Ratchet Message: " + *msg)
			}
		} else {
			go signal.sendWhisperMessage(packet, defaultTTL, packet.RatchetMessage.Header.Destination)
		}
	}
}

// process x3dh message
func (signal *SignalHandler) processDHMessages() {
	for packet := range SignalPacketChannels[messageDH] {

		//gossiper.routingHandler.updateRoutingTable(extPacket.Packet.X3DHMessage.RMessage.Header.Destination, "", 0, extPacket.SenderAddr)

		if packet.X3DHMessage.RMessage.Header.Destination == signal.g.Name {

			value, ok := signal.identityChannels.Load(gossiper.GetKeyFromString(packet.X3DHMessage.RMessage.Header.Origin))

			if ok {
				channel := value.(chan *X3DHMessage)

				go func(c chan *X3DHMessage, d *X3DHMessage) {
					c <- d
				}(channel, packet.X3DHMessage)
			} else {
				// some initiated with old info
			}

		} else {
			go signal.sendWhisperMessage(packet, defaultTTL, packet.X3DHMessage.RMessage.Header.Destination)
		}
	}
}

// Store a new identity
func storeIdentity(selfIdentity SelfIdentity, name string) {
	tmpIdentity := CompressIdentity(selfIdentity)

	b, err := json.Marshal(tmpIdentity)

	if _, err := os.Stat(signalFolder); os.IsNotExist(err) {
		err = os.MkdirAll(signalFolder, 0755)
		if err != nil {
			panic(err)
		}
	}

	file, err := os.Create(signalFolder + name + identityFile)
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}
	defer file.Close()

	_, err = file.Write(b)
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

// Retrieve the identity
func retrieveIdentity(name string) *SelfIdentity {
	file, err := os.Open(signalFolder + name + identityFile)
	if err != nil {
		return nil
	}
	defer file.Close()

	b, errR := ioutil.ReadAll(file)
	helpers.ErrorCheck(errR, false)
	if errR != nil {
		return nil
	}

	selfIdentityCompressed := SelfIdentityCompressed{}
	e := json.Unmarshal(b, &selfIdentityCompressed)
	helpers.ErrorCheck(e, false)
	if e != nil {
		return nil
	}

	selfIdentity := UncompressIdentity(selfIdentityCompressed)

	return &selfIdentity
}

// Start X3DH protocol in sender side
func (signal *SignalHandler) sStartX3DH(user, message *string) {
	compressedIK := CompressPoint(*signal.selfIdentity.IK.PubKey)
	compressedSPK := CompressPoint(*signal.selfIdentity.SPK.PubKey)

	OPK := GenerateDH()
	compressedOPK := CompressPoint(*OPK.PubKey)

	identityMsg := X3DHIdentity{
		Origin:      signal.g.Name,
		Destination: "",
		HopLimit:    uint32(hopLimit),
		IK:          compressedIK,
		SPK:         compressedSPK,
		Sign:        signal.selfIdentity.Sign,
		OPK:         &compressedOPK,
	}

	if user != nil {
		identityMsg.Destination = *user
	}

	go signal.sendIdentity(identityMsg, *OPK, message)
}

// Send a new identity and wait for a X3DHMessage reply
func (signal *SignalHandler) sendIdentity(identityMsg X3DHIdentity, OPK DHPair, message *string) bool {

	//addr := address
	//str := ""
	//if identityMsg.Destination != "" {
	//	signal.routingHandler.mutex.RLock()
	//	addressInTable, isPresent := signal.routingHandler.routingTable[identityMsg.Destination]
	//	signal.routingHandler.mutex.RUnlock()
	//
	//	if isPresent {
	//		addr = addressInTable
	//		str = identityMsg.Destination
	//	}
	//} else {
	//	str = address.IP.String() + ":" + strconv.Itoa(address.Port)
	//}

	fmt.Println("IDENTITY - Origin: " + identityMsg.Origin + " IK: " + string(identityMsg.IK) + " SPK: " + string(identityMsg.SPK) + " OPK: " + string(*identityMsg.OPK) + " Sign: " + fmt.Sprint(identityMsg.Sign.R) + ":" + fmt.Sprint(identityMsg.Sign.S))

	value, _ := signal.identityChannels.LoadOrStore(gossiper.GetKeyFromString(identityMsg.Destination), make(chan *X3DHMessage))
	replyChan := value.(chan *X3DHMessage)

	signal.sendWhisperMessage(&SignalPacket{X3DHIdentity: &identityMsg}, defaultTTL, identityMsg.Destination)

	requestTimer := time.NewTicker(time.Duration(identityTimeout) * time.Second)
	defer requestTimer.Stop()
	retry := 3

	for {
		select {
		case replyPacket := <-replyChan:
			signal.mutex.Lock()
			res := RInitializeX3DH(*replyPacket, &OPK, &signal.selfIdentity, signal.stateTable)
			signal.mutex.Unlock()

			// Send the original message upon both ratchet initialized
			if *res == signal.g.Name && message != nil {
				signal.sendPrivateRatchet(*message, replyPacket.RMessage.Header.Origin)
				return true
			}

			fmt.Println("Unable to initialize X3DH in remote side")
			return false

		case <-requestTimer.C:
			if retry-1 == 0 {
				// store in storage
				return false
			}

			OPK = *GenerateDH()
			opkCompressed := CompressPoint(*OPK.PubKey)
			identityMsg.OPK = &opkCompressed
			signal.sendWhisperMessage(&SignalPacket{X3DHIdentity: &identityMsg}, defaultTTL, identityMsg.Destination)

			retry--
		}
	}
}

// Start X3DH protocol in receiver side
func (signal *SignalHandler) rStartX3DH(rIdentity *X3DHIdentity) {
	remoteIdentity := RemoteIdentity{
		IK:   UncompressPoint(rIdentity.IK),
		SPK:  UncompressPoint(rIdentity.SPK),
		Sign: rIdentity.Sign,
	}

	fmt.Println(rIdentity)
	//fmt.Println("IDENTITY - Origin: " + rIdentity.Origin + " IK: " + string(rIdentity.IK) + " SPK: " + string(rIdentity.SPK) + " OPK: " + string(*rIdentity.OPK) + " Sign: " + fmt.Sprint(rIdentity.Sign.R) + ":" + fmt.Sprint(rIdentity.Sign.S))

	opk := UncompressPoint(*rIdentity.OPK)

	signal.mutex.Lock()
	msg := SInitializeX3DH(remoteIdentity, &opk, signal.g.Name, rIdentity.Origin, &signal.selfIdentity, signal.stateTable)
	signal.mutex.Unlock()

	//signal.routingHandler.mutex.RLock()
	//addressInTable, isPresent := signal.routingHandler.routingTable[rIdentity.Origin]
	//signal.routingHandler.mutex.RUnlock()

	fmt.Println(msg)
	//fmt.Println("X3DH - IK: " + string(msg.IK) + " EK: " + string(msg.EK) + " OPK: " + string(*msg.OPK) + " PubKey: " + string(msg.RMessage.Header.PubKey) + " Pn:" + strconv.FormatInt(msg.RMessage.Header.Pn, 10) + " N: " + strconv.FormatInt(msg.RMessage.Header.N, 10))

	signal.sendWhisperMessage(&SignalPacket{X3DHMessage: msg}, defaultTTL, rIdentity.Origin)

	//if isPresent {
	//	signal.ConnectionHandler.SendPacket(&gossiper.GossipPacket{X3DHMessage: msg}, addressInTable)
	//}

	// maybe retry
}

// Check if I have an initialized ratchet for a given peer
func (signal *SignalHandler) haveRatchet(destination string) (*DRatchetState, bool) {
	signal.mutex.RLock()
	defer signal.mutex.RUnlock()
	return signal.stateTable[destination], signal.stateTable[destination] != nil
}

// Send new private message using the ratchet or start the X3DH protocol in sender side
func (signal *SignalHandler) sendPrivateRatchet(message, destination string) {
	ratchetState, hasRatchet := signal.haveRatchet(destination)
	if hasRatchet {
		//signal.routingHandler.mutex.RLock()
		//addressInTable, isPresent := signal.routingHandler.routingTable[destination]
		//signal.routingHandler.mutex.RUnlock()
		//if isPresent {
		var bytes []byte
		ratchetMessage := ratchetState.RatchetEncrypt(message, signal.g.Name, destination, bytes)
		fmt.Println(ratchetMessage)
		StoreRatchet(*ratchetState, signal.g.Name, destination)
		signal.sendWhisperMessage(&SignalPacket{RatchetMessage: &ratchetMessage}, defaultTTL, destination)
		//}
	} else {
		signal.sStartX3DH(&destination, &message)
	}
}

// Receive a new private message commit from a ratchet
func (signal *SignalHandler) receivePrivateRatchet(ratchetMessage RatchetMessage) *string {
	ratchetState, hasRatchet := signal.haveRatchet(ratchetMessage.Header.Origin)

	if hasRatchet {
		plaintext := ratchetState.RatchetDecrypt(ratchetMessage.Header, ratchetMessage.Message)
		StoreRatchet(*ratchetState, signal.g.Name, ratchetMessage.Header.Origin)
		return plaintext
	} else {
		return nil
	}
}

// Retrieve stored ratchets
func retrieveRatchets(name string, states map[string]*DRatchetState) {
	files, err := ioutil.ReadDir(signalFolder)
	if err != nil {
		log.Fatal(err)
	}

	var names []string

	for _, f := range files {
		if strings.HasSuffix(f.Name(), name+RatchetFile) {
			names = append(names, f.Name())
		}
	}

	var ratchet *DRatchetState
	for _, n := range names {
		ratchet = RetrieveRatchet(n)
		states[ratchet.Peer] = ratchet
	}
}
