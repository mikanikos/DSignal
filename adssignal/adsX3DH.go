package adssignal

import (
	"fmt"
	"strings"
	"bytes"
)

// DEFAULTOPKN number of OPKs generated
var DEFAULTOPKN = 10

// SelfIdentity Peer identity
type SelfIdentity struct {
	IK   DHPair
	SPK  DHPair
	Sign *ECDSA
	OPK []DHPair
}

// SelfIdentityCompressed Peer identity compressed
type SelfIdentityCompressed struct {
	IKPrivKey []byte
	IKPubKey []byte
	SPKPrivKey []byte
	SPKPubKey []byte
	R []byte
	S []byte
	OPK [][][]byte
}

// RemoteIdentity Any remote Peer identity
type RemoteIdentity struct {
	IK   EllipticPoint
	SPK  EllipticPoint
	Sign *ECDSA
	OPK []EllipticPoint
}

// RemoteIdentityCompressed Any remote Peer identity
type RemoteIdentityCompressed struct {
	IK   []byte
	SPK  []byte
	R []byte
	S []byte
	OPK [][]byte
}

// X3DHMessage message
type X3DHMessage struct {
	RMessage RatchetMessage
	IK       []byte
	EK       []byte
	OPK      *[]byte
}

// X3DHIdentity identity
type X3DHIdentity struct {
	Origin      string
	Destination string
	HopLimit    uint32
	IK          []byte
	SPK         []byte
	Sign        *ECDSA
	OPK         *[]byte
}

// GenerateIdentity a new identity
func GenerateIdentity() *SelfIdentity {
	IK := GenerateDH()
	SPK := GenerateDH()

	compressedSPK := Marshal(SPK.PubKey.x, SPK.PubKey.y)
	Sign := GenerateSignature(compressedSPK, IK.PrivKey, *IK.PubKey)

	var OPKS []DHPair
	for i:=0; i<DEFAULTOPKN; i++ {
		OPKS = append(OPKS, *GenerateDH())
	}

	return &SelfIdentity{
		IK: *IK, SPK: *SPK, Sign: Sign, OPK: OPKS,
	}
}

// UpdateIdentity my identity given X time
func (identity *SelfIdentity) UpdateIdentity() {
	SPK := GenerateDH()

	compressedSPK := Marshal(SPK.PubKey.x, SPK.PubKey.y)
	Sign := GenerateSignature(compressedSPK, identity.IK.PrivKey, *identity.IK.PubKey)

	identity.SPK = *SPK
	identity.Sign = Sign
}

// SInitializeX3DH the X3DH protocol in sender side
func SInitializeX3DH(rIdentity RemoteIdentity, oneTimePk *EllipticPoint, sender, receiver string, selfIdentity *SelfIdentity, stateTable map[string]*DRatchetState, message *string) *X3DHMessage {
	compressedSPK := Marshal(rIdentity.SPK.x, rIdentity.SPK.y)

	if !VerifySignature(compressedSPK, *rIdentity.Sign, rIdentity.IK) {
		fmt.Println("Signature verification error")
		return nil
	}
	// Compute dh1, dh2, dh3, dh4 using own and remote identity's keys
	EK := GenerateDH()

	dh1 := DH(selfIdentity.IK, rIdentity.SPK)
	dh2 := DH(*EK, rIdentity.IK)
	dh3 := DH(*EK, rIdentity.SPK)

	dhx := append(dh1, dh2...)
	dhx = append(dhx, dh3...)

	if oneTimePk != nil {
		dh4 := DH(*EK, *oneTimePk)
		dhx = append(dhx, dh4...)
	}

	SK := KdfX3DH(dhx)

	rIKcompressed := Marshal(rIdentity.IK.x, rIdentity.IK.y)
	sIKcompressed := Marshal(selfIdentity.IK.PubKey.x, selfIdentity.IK.PubKey.y)

	AD := append(sIKcompressed, rIKcompressed...)

	EKcompressed := Marshal(EK.PubKey.x, EK.PubKey.y)

	// Initialize the ratchet and create the X3DHMessage including a RatchetMessage
	ratchetState := DRatchetState{}
	ratchetState.SInitializeRatchet(SK, &rIdentity.SPK, receiver)
	stateTable[receiver] = &ratchetState

	msg := sender+":"+receiver
	if message != nil {
		msg = msg + ":" + *message
	}

	ratchetMessage := ratchetState.RatchetEncrypt(msg, sender, receiver, AD)
	x3dhMessage := X3DHMessage{
		ratchetMessage,
		sIKcompressed,
		EKcompressed,
		nil,
	}

	if oneTimePk != nil {
		oneTimePkcompressed := Marshal(oneTimePk.x, oneTimePk.y)
		x3dhMessage.OPK = &oneTimePkcompressed
	}

	return &x3dhMessage
}

// RInitializeX3DH X3DH protocol in receiver side
func RInitializeX3DH(msg X3DHMessage, OPK *DHPair, selfIdentity *SelfIdentity, stateTable map[string]*DRatchetState) *string {
	xIK, yIK, C := Unmarshal(msg.IK)
	rIK := EllipticPoint{
		C, xIK, yIK,
	}

	// Compute dh1, dh2, dh3, dh4 using own and remote identity's keys
	xEK, yEK, C := Unmarshal(msg.EK)
	rEK := EllipticPoint{
		C, xEK, yEK,
	}

	dh1 := DH(selfIdentity.SPK, rIK)
	dh2 := DH(selfIdentity.IK, rEK)
	dh3 := DH(selfIdentity.SPK, rEK)

	dhx := append(dh1, dh2...)
	dhx = append(dhx, dh3...)

	if msg.OPK != nil && OPK != nil {
		dh4 := DH(*OPK, rEK)
		dhx = append(dhx, dh4...)
	}

	SK := KdfX3DH(dhx)

	// Initialize the ratchet and decrypt the message
	ratchetState := DRatchetState{}
	ratchetState.RInitializeRatchet(SK, &selfIdentity.SPK)

	plaintext := ratchetState.RatchetDecrypt(msg.RMessage.Header, msg.RMessage.Message)

	if plaintext == nil {
		fmt.Println("Plaintext non decypted")
		return nil
	}

	plainarray := strings.Split(*plaintext, ":")
	ratchetState.Peer = plainarray[0]
	stateTable[plainarray[0]] = &ratchetState

	return &plainarray[1]
}

// IInitializeX3DH X3DH protocol in receiver side
func IInitializeX3DH(msg X3DHMessage, selfIdentity *SelfIdentity, stateTable map[string]*DRatchetState) *string {
	xIK, yIK, C := Unmarshal(msg.IK)
	rIK := EllipticPoint{
		C, xIK, yIK,
	}

	// Compute dh1, dh2, dh3, dh4 using own and remote identity's keys
	xEK, yEK, C := Unmarshal(msg.EK)
	rEK := EllipticPoint{
		C, xEK, yEK,
	}

	dh1 := DH(selfIdentity.SPK, rIK)
	dh2 := DH(selfIdentity.IK, rEK)
	dh3 := DH(selfIdentity.SPK, rEK)

	dhx := append(dh1, dh2...)
	dhx = append(dhx, dh3...)

	if msg.OPK != nil {
		var OPK *DHPair
		for _, op := range selfIdentity.OPK {
			if bytes.Compare(CompressPoint(*op.PubKey), *msg.OPK) == 0 {
				OPK = &op
				break
			}
		}

		if OPK != nil {
			dh4 := DH(*OPK, rEK)
			dhx = append(dhx, dh4...)
		}
	}

	SK := KdfX3DH(dhx)

	// Initialize the ratchet and decrypt the message
	ratchetState := DRatchetState{}
	ratchetState.RInitializeRatchet(SK, &selfIdentity.SPK)

	plaintext := ratchetState.RatchetDecrypt(msg.RMessage.Header, msg.RMessage.Message)

	if plaintext == nil {
		fmt.Println("Plaintext non decypted")
		return nil
	}

	plainarray := strings.Split(*plaintext, ":")
	ratchetState.Peer = plainarray[0]
	stateTable[plainarray[0]] = &ratchetState

	return &plainarray[2]
}

// CompressIdentity identity
func CompressIdentity(selfIdentity SelfIdentity) SelfIdentityCompressed {
	IKcompressed := CompressPoint(*selfIdentity.IK.PubKey)
	SPKcompressed := CompressPoint(*selfIdentity.SPK.PubKey)

	var OPK [][][]byte
	var OPKpair [][]byte
	for i:=0; i<DEFAULTOPKN; i++ {
		OPKpair = [][]byte {*selfIdentity.OPK[i].PrivKey.d, CompressPoint(*selfIdentity.OPK[i].PubKey)}
		OPK = append(OPK, OPKpair)
	}

	return SelfIdentityCompressed {
		*selfIdentity.IK.PrivKey.d,
		IKcompressed,
		*selfIdentity.SPK.PrivKey.d,
		SPKcompressed,
		selfIdentity.Sign.R,
		selfIdentity.Sign.S,
		OPK,
	}
}

// CompressRemoteIdentity identity
func CompressRemoteIdentity(selfIdentity SelfIdentityCompressed) RemoteIdentityCompressed {
	var OPK [][]byte
	for i:=0; i<DEFAULTOPKN; i++ {
		OPK = append(OPK, selfIdentity.OPK[i][1])
	}

	return RemoteIdentityCompressed {
		selfIdentity.IKPubKey,
		selfIdentity.SPKPubKey,
		selfIdentity.R,
		selfIdentity.S,
		OPK,
	}
}

// UncompressIdentity identity
func UncompressIdentity(compressed SelfIdentityCompressed) SelfIdentity {

	IKPrivKey := PrivateKey { &compressed.IKPrivKey }
	SPKPrivKey := PrivateKey { &compressed.SPKPrivKey }
	IKPubKey := UncompressPoint(compressed.IKPubKey)
	SPKPubKey := UncompressPoint(compressed.SPKPubKey)

	IK := DHPair{
		&IKPubKey,
		&IKPrivKey,
	}

	SPK := DHPair{
		&SPKPubKey,
		&SPKPrivKey,
	}

	sign := ECDSA {
		compressed.R,
		compressed.S,
	}

	var OPK []DHPair
	var OPKPrivKey PrivateKey
	var OPKPubKey EllipticPoint
	for i:=0; i<DEFAULTOPKN; i++ {
		OPKPrivKey = PrivateKey { &compressed.OPK[i][0] }
		OPKPubKey = UncompressPoint(compressed.OPK[i][1])
		OPK = append(OPK, DHPair{
			&OPKPubKey,
			&OPKPrivKey,
		})
	}

	return SelfIdentity {
		IK,
		SPK,
		&sign,
		OPK,
	}
}

// UncompressRemoteIdentity identity
func UncompressRemoteIdentity(compressed RemoteIdentityCompressed) RemoteIdentity {

	IK := UncompressPoint(compressed.IK)
	SPK := UncompressPoint(compressed.SPK)

	sign := ECDSA {
		compressed.R,
		compressed.S,
	}

	var OPK []EllipticPoint
	for i:=0; i<DEFAULTOPKN; i++ {
		OPK = append(OPK, UncompressPoint(compressed.OPK[i]))
	}

	return RemoteIdentity {
		IK,
		SPK,
		&sign,
		OPK,
	}
}