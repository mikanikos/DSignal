package whisper

import (
	"fmt"
	ecies "github.com/ecies/go"
	"sync"
)

// Filter is a message filter
type Filter struct {
	KeyAsym *ecies.PrivateKey
	KeySym  []byte
	Pow     float64
	Topics  [][]byte

	Messages map[[32]byte]*ReceivedMessage
	Mutex    sync.RWMutex
}

// FilterStorage stores all the filters created
type FilterStorage struct {
	subscribers map[string]*Filter
	topicToFilters map[Topic]map[*Filter]struct{}
	mutex sync.RWMutex
}

// NewFilterStorage returns a newly created filter collection
func NewFilterStorage() *FilterStorage {
	return &FilterStorage{
		subscribers:    make(map[string]*Filter),
		topicToFilters: make(map[Topic]map[*Filter]struct{}),
		//allTopicsMatcher: make(map[*Filter]struct{}),
		//whisper:          w,
	}
}

// AddFilter will handleEnvelope a new filter to the filter collection
func (fs *FilterStorage) AddFilter(filter *Filter) (string, error) {
	if filter.KeySym != nil && filter.KeyAsym != nil {
		return "", fmt.Errorf("filters must choose between symmetric and asymmetric keys")
	}

	if filter.Messages == nil {
		filter.Messages = make(map[[32]byte]*ReceivedMessage)
	}

	id, err := GenerateRandomID()
	if err != nil {
		return "", err
	}

	fs.mutex.Lock()

	_, loaded := fs.subscribers[id]
	if loaded {
		return "", fmt.Errorf("failed to generate unique ID")
	}

	fs.subscribers[id] = filter
	for _, t := range filter.Topics {
		topic := ConvertBytesToTopic(t)
		if fs.topicToFilters[topic] == nil {
			fs.topicToFilters[topic] = make(map[*Filter]struct{})
		}
		fs.topicToFilters[topic][filter] = struct{}{}
	}

	fs.mutex.Unlock()

	return id, err
}

// RemoveFilter will remove a filter from id given
func (fs *FilterStorage) RemoveFilter(id string) bool {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	sub, loaded := fs.subscribers[id]
	if loaded {
		delete(fs.subscribers, id)
		for _, topic := range sub.Topics {
			delete(fs.topicToFilters[ConvertBytesToTopic(topic)], sub)
		}
		return true
	}
	return false
}

// getSubscribersByTopic returns all filters given a topic
func (fs *FilterStorage) getSubscribersByTopic(topic Topic) []*Filter {
	res := make([]*Filter, 0, len(fs.topicToFilters[topic]))
	for subscriber := range fs.topicToFilters[topic] {
		res = append(res, subscriber)
	}
	return res
}

// GetFilterFromID returns a filter from the collection with a specific ID
func (fs *FilterStorage) GetFilterFromID(id string) *Filter {
	fs.mutex.RLock()
	defer fs.mutex.RUnlock()
	return fs.subscribers[id]
}

// NotifySubscribers notifies filters of matching envelope topic
func (fs *FilterStorage) NotifySubscribers(env *Envelope) {
	var msg *ReceivedMessage

	fs.mutex.RLock()
	defer fs.mutex.RUnlock()

	candidates := fs.getSubscribersByTopic(env.Topic)
	for _, sub := range candidates {
		if sub.Pow <= 0 || env.pow >= sub.Pow {
			msg = env.GetMessageFromEnvelope(sub)
			if msg == nil {
				fmt.Println("failed to open message")
			} else {
				sub.Mutex.Lock()
				if _, exist := sub.Messages[msg.EnvelopeHash]; !exist {
					sub.Messages[msg.EnvelopeHash] = msg
				}
				sub.Mutex.Unlock()
			}
		}
	}
}

func (f *Filter) isAsymmetricEncryption() bool {
	return f.KeyAsym != nil
}

func (f *Filter) isSymmetricEncryption() bool {
	return f.KeySym != nil
}

// GetReceivedMessagesFromFilter returns received messages given for a filter
func (f *Filter) GetReceivedMessagesFromFilter() (messages []*ReceivedMessage) {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	messages = make([]*ReceivedMessage, 0, len(f.Messages))
	for _, msg := range f.Messages {
		messages = append(messages, msg)
	}

	f.Messages = make(map[[32]byte]*ReceivedMessage)
	return messages
}