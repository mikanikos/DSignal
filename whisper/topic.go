/* Contributors: Andrea Piccione */

package whisper

import "encoding/hex"

// Topic categorize messages
type Topic [TopicLength]byte

// ConvertBytesToTopic converts from byte array to Topic
func ConvertBytesToTopic(b []byte) (t Topic) {
	sz := TopicLength
	if x := len(b); x < TopicLength {
		sz = x
	}
	for i := 0; i < sz; i++ {
		t[i] = b[i]
	}
	return t
}

// String converts a topic byte array to a string
func (t *Topic) String() string {
	return hex.EncodeToString(t[:])
}

// ConvertTopicToBloom converts the topic (4 bytes) to the bloom filter (64 bytes)
func ConvertTopicToBloom(topic Topic) []byte {
	b := make([]byte, BloomFilterSize)
	var index [3]int
	for j := 0; j < 3; j++ {
		index[j] = int(topic[j])
		if (topic[3] & (1 << uint(j))) != 0 {
			index[j] += 256
		}
	}

	for j := 0; j < 3; j++ {
		byteIndex := index[j] / 8
		bitIndex := index[j] % 8
		b[byteIndex] = (1 << uint(bitIndex))
	}
	return b
}