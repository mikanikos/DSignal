// Authors: Dewmini Marakkalage

package storage

// DStoreMessage encloses various othe messages used by a DStore
type DStoreMessage struct {
	SourceID []byte
	RPCID    string

	PingRequest      *PingRequest
	PingReply        *PingReply
	FindNodeRequest  *FindNodeRequest
	FindNodeReply    *FindNodeReply
	StoreRequest     *StoreRequest
	StoreReply       *StoreReply
	FindValueRequest *FindValueRequest
	FindValueReply   *FindValueReply
}

// PingRequest asks whether the receiving node is alive.
type PingRequest struct {
}

// NewPingRequest ...
func NewPingRequest() *DStoreMessage {
	return &DStoreMessage{PingRequest: &PingRequest{}}
}

// PingReply is sent when a PingRequest is received.
type PingReply struct {
}

// FindNodeRequest asks for the replicationParam many nodes whose IDs are closest to a given ID.
type FindNodeRequest struct {
	Target []byte
}

// NewFindNodeRequest ...
func NewFindNodeRequest(target []byte) *DStoreMessage {
	return &DStoreMessage{FindNodeRequest: &FindNodeRequest{Target: target}}
}

// FindNodeReply is sent when a FindNodeRequest is received.
type FindNodeReply struct {
	Nodes []*RoutingEntry
}

// StoreRequest asks to store a given key-value pair.
type StoreRequest struct {
	Key           []byte // key to store
	Value         []byte // value to store
	ExpiryTime    uint64 // expiry timeout in seconds
	RePublishTime uint64 // republish timeout in secons
}

// NewStoreRequest ...
func NewStoreRequest(key []byte, value []byte, te uint64, tr uint64) *DStoreMessage {
	return &DStoreMessage{StoreRequest: &StoreRequest{Key: key, Value: value, ExpiryTime: te, RePublishTime: tr}}
}

// StoreReply is sent when a StoreRequest is received.
type StoreReply struct {
	Key   []byte
	Value []byte
}

// FindValueRequest ask to find a value of a given key or return replicationParam many nodes whose IDs are closest to that key.
type FindValueRequest struct {
	Key []byte
}

// NewFindValueRequest ...
func NewFindValueRequest(key []byte) *DStoreMessage {
	return &DStoreMessage{FindValueRequest: &FindValueRequest{Key: key}}
}

// FindValueReply is send when a FindValueRequest is received.
type FindValueReply struct {
	Key   []byte
	Value []byte
	Nodes []*RoutingEntry
}
