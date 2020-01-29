package adssignal

import (
	"encoding/json"
	"os"
	"io/ioutil"
	"fmt"
	"crypto/sha256"
	"encoding/hex"
	"encoding/binary"

	"github.com/mikanikos/DSignal/helpers"
)

// DRatchetState State one per connection
type DRatchetState struct {
	Peer string
	DHs      *DHPair
	DHr      *EllipticPoint
	RK       *[]byte
	CKs, CKr *[]byte
	Ns, Nr   int64
	PN       int64
	MKSKIPPED map[string][]byte
}

// DRatchetStateCompressed ratchet compressed
type DRatchetStateCompressed struct {
	Peer string
	DHsPrivKey,	DHsPubKey	[]byte
	DHr      []byte
	RK       []byte
	CKs, CKr []byte
	Ns, Nr   int64
	PN       int64
	MKSKIPPED map[string][]byte
}

// RatchetFile const
var RatchetFile = "Ratchet.txt"
var ratchetHopLimit = 10
// MaxSkip const
var MaxSkip = 10

// RatchetCipher ciphertext
type RatchetCipher struct {
	Ad   []byte
	Ciph []byte
}

// RatchetHeader header
type RatchetHeader struct {
	PubKey      []byte
	Pn          int64
	N           int64
	Origin      string
	Destination string
	HopLimit    uint32
}

// RatchetMessage with both the cipher and the header
type RatchetMessage struct {
	Header  RatchetHeader
	Message RatchetCipher
}

// SInitializeRatchet ratchet in the sending member
func (state *DRatchetState) SInitializeRatchet(SK []byte, pubKey *EllipticPoint, peer string) {
	state.Peer = peer
	state.DHs = GenerateDH()
	state.DHr = pubKey
	state.RK, state.CKs = KdfRK(SK, DH(*state.DHs, *state.DHr))
	state.CKr = nil
	state.Ns = 0
	state.Nr = 0
	state.PN = 0
	state.MKSKIPPED = make(map[string][]byte)
}

// RInitializeRatchet ratchet in the receiving member
func (state *DRatchetState) RInitializeRatchet(SK []byte, dhPair *DHPair) {
	state.DHs = dhPair
	state.DHr = nil
	state.RK = &SK
	state.CKs = nil
	state.CKr = nil
	state.Ns = 0
	state.Nr = 0
	state.PN = 0
	state.MKSKIPPED = make(map[string][]byte)
}

// RatchetEncrypt using the ratchet state
func (state *DRatchetState) RatchetEncrypt(plaintext, origin, destination string, AD []byte) RatchetMessage {
	var mk *[]byte
	state.CKs, mk = KdfCK(*state.CKs)

	ratchetHeader := ComposeRatchetHeader(*state.DHs, state.PN, state.Ns, origin, destination)
	concatADH := Concat(AD, ratchetHeader)
	ratchetCipher := EncryptAEAD(*mk, plaintext, concatADH)

	state.Ns++

	return RatchetMessage{
		ratchetHeader,
		ratchetCipher,
	}
}

// RatchetDecrypt using the ratchet state
func (state *DRatchetState) RatchetDecrypt(header RatchetHeader, cipher RatchetCipher) *string {
	plaintext := state.TrySkippedMessageKeys(header, cipher)

	if plaintext != nil {
		return plaintext
	}

	x, y, curve := Unmarshal(header.PubKey)

	rcvPubKey := EllipticPoint{
		curve, x, y,
	}

	if state.DHr == nil || !ComparePoints(rcvPubKey, *state.DHr) {
		state.SkipMessageKeys(header.N)
		state.DHRatchet(header)
	}

	state.SkipMessageKeys(header.N)
	var mk *[]byte
	state.CKr, mk = KdfCK(*state.CKr)
	state.Nr++

	return DecryptAEAD(*mk, cipher, header)
}

// TrySkippedMessageKeys try out of order keys
func (state *DRatchetState) TrySkippedMessageKeys(header RatchetHeader, cipher RatchetCipher) *string {

	dhr := UncompressPoint(header.PubKey)
	
	if val, ok := state.MKSKIPPED[getKeyFromRatchet(dhr, header.N)]; ok {
		mk := val
		delete(state.MKSKIPPED, getKeyFromRatchet(dhr, header.N))

		return DecryptAEAD(mk, cipher, header)
	}

	return nil
}

// SkipMessageKeys skip keys out of order
func (state *DRatchetState) SkipMessageKeys(until int64) {
	if state.Nr + int64(MaxSkip) < until {
		fmt.Println("Too many skipped keys error")
	}

	if state.CKr != nil {
		for state.Nr < until {
			ckr, mk := KdfCK(*state.CKr)
			state.CKr = ckr
			state.MKSKIPPED[getKeyFromRatchet(*state.DHr, state.Nr)] = *mk
			state.Nr++
		}
	}
}

// DHRatchet a step of Diffie-Hellman Ratchet
func (state *DRatchetState) DHRatchet(header RatchetHeader) {
	state.PN = state.Ns
	state.Ns = 0
	state.Nr = 0

	x, y, curve := Unmarshal(header.PubKey)

	rcvPubKey := EllipticPoint{
		curve, x, y,
	}

	state.DHr = &rcvPubKey
	state.RK, state.CKr = KdfRK(*state.RK, DH(*state.DHs, *state.DHr))
	state.DHs = GenerateDH()
	state.RK, state.CKs = KdfRK(*state.RK, DH(*state.DHs, *state.DHr))
}

// ComposeRatchetHeader the Ratchet Header
func ComposeRatchetHeader(dhPair DHPair, pn, n int64, origin, destination string) RatchetHeader {
	pubKey := Marshal((*dhPair.PubKey).x, (*dhPair.PubKey).y)
	return RatchetHeader{
		pubKey,
		pn,
		n,
		origin,
		destination,
		uint32(ratchetHopLimit),
	}
}

// StoreRatchet a new ratchet
func StoreRatchet(ratchet DRatchetState, origin, destination string) {
	tmpRatchet := CompressRatchet(ratchet)

	b, err := json.Marshal(tmpRatchet)

	if _, err := os.Stat(signalFolder); os.IsNotExist(err) {
			err = os.MkdirAll(signalFolder, 0755)
			if err != nil {
					panic(err)
			}
	}

	file, err := os.Create(signalFolder + destination + "_" + origin + RatchetFile)
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

// RetrieveRatchet the ratchet
func RetrieveRatchet(name string) *DRatchetState {
	file, err := os.Open(signalFolder + name)
    if err != nil {
        return nil
    }
    defer file.Close()

	b, errR := ioutil.ReadAll(file)
	helpers.ErrorCheck(errR, false)
	if errR != nil {
		return nil
	}
	
	ratchetCompressed := DRatchetStateCompressed{}
	e := json.Unmarshal(b, &ratchetCompressed)
	helpers.ErrorCheck(e, false)
	if e != nil {
		return nil
	}

	ratchet := UncompressRatchet(ratchetCompressed)

	return &ratchet
}

// CompressRatchet ratchet state
func CompressRatchet(ratchet DRatchetState) DRatchetStateCompressed {
	DHsCompressed := CompressPoint(*ratchet.DHs.PubKey)
	DHrCompressed := CompressPoint(*ratchet.DHr)

	return DRatchetStateCompressed{
		ratchet.Peer,
		*ratchet.DHs.PrivKey.d,
		DHsCompressed,
		DHrCompressed,
		*ratchet.RK,
		*ratchet.CKs,
		*ratchet.CKr,
		ratchet.Ns, 
		ratchet.Nr,
		ratchet.PN,
		ratchet.MKSKIPPED,
	}
}

// UncompressRatchet ratchet state
func UncompressRatchet(compressed DRatchetStateCompressed) DRatchetState {
	DHsPrivKey := PrivateKey { &compressed.DHsPrivKey }
	DHsPubKey := UncompressPoint(compressed.DHsPubKey)
	DHr := UncompressPoint(compressed.DHr)

	DHs := DHPair{
		&DHsPubKey,
		&DHsPrivKey,
	}

	return DRatchetState {
		compressed.Peer,
		&DHs,
		&DHr,
		&compressed.RK,
		&compressed.CKs, &compressed.CKr,
		compressed.Ns,
		compressed.Nr,
		compressed.PN,
		compressed.MKSKIPPED,
	}
}

func getKeyFromRatchet(DHr EllipticPoint, Nr int64) string {
	DHrCompressed := CompressPoint(DHr)
	hasher := sha256.New()
	hasher.Write(DHrCompressed)
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(Nr))
	return hex.EncodeToString(hasher.Sum(b))
}