package adssignal

import (
	"encoding/json"
	"encoding/hex"
	"sync"
	"time"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"math/rand"
	
	"github.com/dedis/protobuf"
	"github.com/mikanikos/DSignal/gossiper"
	"github.com/mikanikos/DSignal/whisper"
	"github.com/mikanikos/DSignal/helpers"
	"github.com/mikanikos/DSignal/storage"
)

const (
	signalFolder 	   = "_SignalFolder/"
	identityFile       = "Identity.txt"
	identityTimeout    = 3
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

// SignalPacketChannels map of signal packets
var SignalPacketChannels map[int]chan *SignalPacket

// SignalHandler structure including self identity, state table, identity channels and a mutex
type SignalHandler struct {
	w                *whisper.Whisper
	g                *gossiper.Gossiper
	selfIdentity     SelfIdentity
	stateTable       map[string]*DRatchetState
	identityChannels sync.Map
	latestMessages map[string][]gossiper.RumorMessage
	identityStr 	 string

	mutex sync.RWMutex
}

// NewSignalHandler Create a new handler
func NewSignalHandler(name string, g *gossiper.Gossiper, w *whisper.Whisper) *SignalHandler {
	identity := retrieveIdentity(name)
	states := make(map[string]*DRatchetState)

	if identity == nil {
		identity = GenerateIdentity()
		storeIdentity(*identity, name)
	} else {
		retrieveRatchets(name, states)
	}

	signalHandler :=  &SignalHandler{
		g: g,
		w: w,
		selfIdentity:     *identity,
		stateTable:       states,
		identityChannels: sync.Map{},
		latestMessages: make(map[string][]gossiper.RumorMessage),
	}
	signalHandler.identityStr = identityMessageSelf(*identity)

	return signalHandler
}

// Run the signal handler
func (signal *SignalHandler) Run() {
	go signal.processIdentityMessages()
	go signal.processDHMessages()
	go signal.processRatchetMessages()

	SignalPacketChannels = make(map[int]chan *SignalPacket)
	for i := identityDH; i <= ratchet; i++ {
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
	go signal.listenForClientMessages()

	tmpIdentity := CompressIdentity(signal.selfIdentity)
	remoteIdentity := CompressRemoteIdentity(tmpIdentity)
	b, err := json.Marshal(remoteIdentity)
	if err != nil {
		fmt.Println(err)
	}
	metaHash := signal.g.DstorageHandler.DStore.StoreFile(b, storage.HashTypeSha256, storage.TypeBasicFile)
	fmt.Println("Identity metahash: " + hex.EncodeToString(metaHash))
}

func (signal *SignalHandler) listenForClientMessages() {
	for message := range gossiper.SignalChannel {
		go signal.sendPrivateRatchet(message.Text, *message.Destination, *message.Identity)
	}
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
						fmt.Println("¿?")
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
		go signal.sendPrivateRatchet(message.Text, *message.Destination, *message.Identity)
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

		if packet.X3DHMessage.RMessage.Header.Destination == signal.g.Name {

			value, ok := signal.identityChannels.Load(gossiper.GetKeyFromString(packet.X3DHMessage.RMessage.Header.Origin))

			if ok {
				channel := value.(chan *X3DHMessage)

				go func(c chan *X3DHMessage, d *X3DHMessage) {
					c <- d
				}(channel, packet.X3DHMessage)
			} else {
				mes := "\nSignal: X3DHMessage from " + packet.X3DHMessage.RMessage.Header.Origin + " including IK:" + hex.EncodeToString(packet.X3DHMessage.IK) 
				mes = mes + " EK:" + hex.EncodeToString(packet.X3DHMessage.EK) + " OPK:" + hex.EncodeToString(*packet.X3DHMessage.OPK) 
				fmt.Println(mes)

				signal.mutex.Lock()
				res := IInitializeX3DH(*packet.X3DHMessage, &signal.selfIdentity, signal.stateTable)
				signal.mutex.Unlock()

				// Send the original message upon both ratchet initialized
				if res != nil {
					mes = "\nSignal: Initialize Diffie-Hellman Ratchet with " + packet.X3DHMessage.RMessage.Header.Origin + " and new public key" + hex.EncodeToString(packet.X3DHMessage.RMessage.Header.PubKey) 
					fmt.Println(mes)

					mes = "\nSignal: Make a Symmetric Ratchet receive step with " + packet.X3DHMessage.RMessage.Header.Origin + " and decipher " + *res
					fmt.Println(mes)
					
					continue
				}

				fmt.Println("Unable to initialize X3DH in remote side")
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

func (signal *SignalHandler) iStartX3DH(user, message *string, rIdentity RemoteIdentity) {
	OPK := rIdentity.OPK[rand.Intn(len(rIdentity.OPK))]

	signal.mutex.Lock()
	res := SInitializeX3DH(rIdentity, &OPK, signal.g.Name, *user, &signal.selfIdentity, signal.stateTable, message)
	signal.mutex.Unlock()

	mes := "\nSignal: Starting X3DH with " + *user + " using OPK:" + hex.EncodeToString(CompressPoint(OPK)) 
	fmt.Println(mes)

	mes = "\nSignal: Initialize Diffie-Hellman Ratchet with " + *user + " using retrieved identity"
	fmt.Println(mes)

	signal.sendWhisperMessage(&SignalPacket{X3DHMessage: res}, defaultTTL, *user)
}

// Send a new identity and wait for a X3DHMessage reply
func (signal *SignalHandler) sendIdentity(identityMsg X3DHIdentity, OPK DHPair, message *string) bool {

	value, _ := signal.identityChannels.LoadOrStore(gossiper.GetKeyFromString(identityMsg.Destination), make(chan *X3DHMessage))
	replyChan := value.(chan *X3DHMessage)

	mes := "\nSignal: Starting X3DH with " + identityMsg.Destination + " using OPK:" + hex.EncodeToString(*identityMsg.OPK) 
	fmt.Println(mes)

	requestTimer := time.NewTicker(time.Duration(identityTimeout) * time.Second)
	defer requestTimer.Stop()
	retry := 3

	signal.sendWhisperMessage(&SignalPacket{X3DHIdentity: &identityMsg}, defaultTTL, identityMsg.Destination)

	for {
		select {
		case replyPacket := <-replyChan:
			requestTimer.Stop()

			mes = "\nSignal: X3DHMessage from " + replyPacket.RMessage.Header.Origin + " including IK:" + hex.EncodeToString(replyPacket.IK) 
			mes = mes + " EK:" + hex.EncodeToString(replyPacket.EK) + " OPK:" + hex.EncodeToString(*replyPacket.OPK) 
			fmt.Println(mes)

			signal.mutex.Lock()
			res := RInitializeX3DH(*replyPacket, &OPK, &signal.selfIdentity, signal.stateTable)
			signal.mutex.Unlock()

			// Send the original message upon both ratchet initialized
			if res != nil && *res == signal.g.Name && message != nil {
				mes = "\nSignal: Initialize Diffie-Hellman Ratchet with " + replyPacket.RMessage.Header.Origin + " and new public key" + hex.EncodeToString(replyPacket.RMessage.Header.PubKey) 
				fmt.Println(mes)

				mes = "\nSignal: Make a Symmetric Ratchet receive step with " + replyPacket.RMessage.Header.Origin + " and decipher " + *res
				fmt.Println(mes)
				
				signal.sendPrivateRatchet(*message, replyPacket.RMessage.Header.Origin, "")
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

	mes := "\nSignal: New identity from" +  rIdentity.Origin + " including IK:" + hex.EncodeToString(rIdentity.IK) 
	mes = mes + " SPK:" + hex.EncodeToString(rIdentity.SPK) + " OPK:" + hex.EncodeToString(*rIdentity.OPK) + " Signature:" + hex.EncodeToString(rIdentity.Sign.R) + "-" + hex.EncodeToString(rIdentity.Sign.S)
	fmt.Println(mes)

	opk := UncompressPoint(*rIdentity.OPK)

	signal.mutex.Lock()
	msg := SInitializeX3DH(remoteIdentity, &opk, signal.g.Name, rIdentity.Origin, &signal.selfIdentity, signal.stateTable, nil)
	signal.mutex.Unlock()

	mes = "\nSignal: Initialize Diffie-Hellman Ratchet with " + rIdentity.Origin + " using identity"
	fmt.Println(mes)

	mes = "\nSignal: Continuing X3DH with " + rIdentity.Origin + " using OPK:" + hex.EncodeToString(*rIdentity.OPK) 
	fmt.Println(mes)

	signal.sendWhisperMessage(&SignalPacket{X3DHMessage: msg}, defaultTTL, rIdentity.Origin)
}

// Check if I have an initialized ratchet for a given peer
func (signal *SignalHandler) haveRatchet(destination string) (*DRatchetState, bool) {
	signal.mutex.RLock()
	defer signal.mutex.RUnlock()
	return signal.stateTable[destination], signal.stateTable[destination] != nil
}

// Send new private message using the ratchet or start the X3DH protocol in sender side
func (signal *SignalHandler) sendPrivateRatchet(message, destination, identity string) {
	ratchetState, hasRatchet := signal.haveRatchet(destination)
	if hasRatchet {
		var bytes []byte
		ratchetMessage := ratchetState.RatchetEncrypt(message, signal.g.Name, destination, bytes)
		
		mes := "\nSignal: Make a Symmetric Ratchet sending step with " + destination + " and sending id " + fmt.Sprint(ratchetMessage.Header.N)
		fmt.Println(mes)

		StoreRatchet(*ratchetState, signal.g.Name, destination)
		signal.latestMessages[destination] = append(signal.latestMessages[destination], gossiper.RumorMessage{ signal.g.Name, uint32(ratchetMessage.Header.N), message })
		signal.sendWhisperMessage(&SignalPacket{RatchetMessage: &ratchetMessage}, defaultTTL, destination)
	} else {
		if identity != "" {
			decoded, err := hex.DecodeString(identity)
			if err != nil {
				log.Fatal(err)
			}

			b := signal.g.DstorageHandler.DStore.RetrieveFile(decoded)
			rIdentityCompressed := RemoteIdentityCompressed{}
			e := json.Unmarshal(b, &rIdentityCompressed)

			if e != nil {
				signal.sStartX3DH(&destination, &message)
				return
			}

			identity := UncompressRemoteIdentity(rIdentityCompressed)
			signal.iStartX3DH(&destination, &message, identity)

		} else {
			signal.sStartX3DH(&destination, &message)
		}
	}
}

// Receive a new private message commit from a ratchet
func (signal *SignalHandler) receivePrivateRatchet(ratchetMessage RatchetMessage) *string {
	mes := "\nSignal: Received new Ratchet message from " + ratchetMessage.Header.Origin 
	fmt.Println(mes)

	ratchetState, hasRatchet := signal.haveRatchet(ratchetMessage.Header.Origin)

	if hasRatchet {
		plaintext := ratchetState.RatchetDecrypt(ratchetMessage.Header, ratchetMessage.Message)

		if plaintext != nil {
			mes = "\nSignal: Make a Diffie-Hellman Ratchet step with " + ratchetMessage.Header.Origin + " and public key " + hex.EncodeToString(ratchetMessage.Header.PubKey)
			fmt.Println(mes)
			mes = "\nSignal: Make a Symmetric Ratchet receving step with " + ratchetMessage.Header.Origin + ", receiving id " + fmt.Sprint(ratchetMessage.Header.N) + " and decipher " + *plaintext
			fmt.Println(mes)

			signal.latestMessages[ratchetMessage.Header.Origin] = append(signal.latestMessages[ratchetMessage.Header.Origin], gossiper.RumorMessage{ ratchetMessage.Header.Origin, uint32(ratchetMessage.Header.N), *plaintext })

			StoreRatchet(*ratchetState, signal.g.Name, ratchetMessage.Header.Origin)
		}

		return plaintext
	}
		
	return nil
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

func identityMessageSelf(selfIdentity SelfIdentity) string {
	compressed := CompressIdentity(selfIdentity)
	message := "$IK%:" + hex.EncodeToString(compressed.IKPubKey) + "$SPK%:" + hex.EncodeToString(compressed.SPKPubKey)
	message = message + "$Signature%:" + hex.EncodeToString(compressed.R) + "-" + hex.EncodeToString(compressed.S)
	message = message + "$OPK%:" + hex.EncodeToString(compressed.OPK[0][1]) + "..."

	return message
}

// GetRatchetMessages util
func  (signal *SignalHandler) GetRatchetMessages(user string) []gossiper.RumorMessage {
	return signal.latestMessages[user]
}

// GetIdentity util
func  (signal *SignalHandler) GetIdentity() string {
	return signal.identityStr
}