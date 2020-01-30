// Contributors: Andrea Piccione

package gossiper

import (
	"encoding/hex"
	"fmt"
)

// POSSIBLE OPTIMIZATION: if current file from our client was not agreed in consensus, should I use it for the next round?

// que sera consensus round
func (gossiper *Gossiper) qscRound(extPacket *ExtendedGossipPacket) {

	// get current round
	roundS := gossiper.blockchainHandler.myTime

	if debug {
		fmt.Println("FITNESS: " + fmt.Sprint(extPacket.Packet.TLCMessage.Fitness))
	}

	if debug {
		fmt.Println("ROUND S")
	}

	// round s
	gossiper.tlcRound(extPacket)

	value, loaded := gossiper.blockchainHandler.confirmations.Load(roundS)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	// get confirmations and highest fitness tlc
	confMapS := value.(*SafeTLCMessagesMap)
	confMapS.Mutex.RLock()
	confirmationsRoundS := confMapS.OriginMessage
	confMapS.Mutex.RUnlock()
	highestTLCRoundS := getTLCWithHighestFitness(confirmationsRoundS)

	if debug {
		fmt.Println("Highest in round s : " + highestTLCRoundS.Origin + " " + fmt.Sprint(highestTLCRoundS.ID) + " " + fmt.Sprint(highestTLCRoundS.Fitness))
	}

	if debug {
		fmt.Println("ROUND S+1")
	}

	// round s + 1
	gossiper.tlcRound(gossiper.createTLCMessage(highestTLCRoundS.TxBlock, -1, highestTLCRoundS.Fitness))
	value, loaded = gossiper.blockchainHandler.confirmations.Load(roundS + 1)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	// get confirmations and highest fitness tlc
	confMapS1 := value.(*SafeTLCMessagesMap)
	confMapS1.Mutex.RLock()
	confirmationsRoundS1 := confMapS1.OriginMessage
	confMapS1.Mutex.RUnlock()

	highestTLCRoundS1 := getTLCWithHighestFitness(confirmationsRoundS1)

	if debug {
		fmt.Println("Highest in round s + 1 : " + highestTLCRoundS1.Origin + " " + fmt.Sprint(highestTLCRoundS1.ID) + " " + fmt.Sprint(highestTLCRoundS1.Fitness))
	}

	if debug {
		fmt.Println("ROUND S+2")
	}

	// round s + 2 (IS IT NECESSARY?)
	gossiper.tlcRound(gossiper.createTLCMessage(highestTLCRoundS1.TxBlock, -1, highestTLCRoundS1.Fitness))
	value, loaded = gossiper.blockchainHandler.confirmations.Load(roundS + 2)
	if !loaded {
		fmt.Println("ERROR: no new round")
		return
	}

	if debug {
		fmt.Println("CHECKING IF CONSENSUS REACHED")
	}

	// get message if consensus reached, otherwise return nil
	if messageConsensus := gossiper.blockchainHandler.checkIfConsensusReached(confirmationsRoundS, confirmationsRoundS1, roundS); messageConsensus != nil {

		if debug {
			fmt.Println("GOT CONSENSUS")
		}

		// if got consensus, update blockchain
		chosenBlock := messageConsensus.TxBlock
		chosenBlock.PrevHash = gossiper.blockchainHandler.topBlockchainHash
		gossiper.blockchainHandler.committedHistory.Store(chosenBlock.Hash(), chosenBlock)
		gossiper.blockchainHandler.topBlockchainHash = chosenBlock.Hash()
		gossiper.blockchainHandler.previousBlockHash = gossiper.blockchainHandler.topBlockchainHash

		// if mine, notify gui
		if messageConsensus.Origin == gossiper.Name {
			go func(b *BlockPublish) {
				gossiper.fileHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
			}(&chosenBlock)
		}

		gossiper.printConsensusMessage(messageConsensus)

	} else {
		// if not consensus, update blockchain with highest tilc from round s + 1
		chosenBlock := highestTLCRoundS1.TxBlock
		gossiper.blockchainHandler.previousBlockHash = chosenBlock.Hash()

		// if mine, notify gui
		if highestTLCRoundS1.Origin == gossiper.Name {
			go func(b *BlockPublish) {
				gossiper.fileHandler.filesIndexed <- &FileGUI{Name: b.Transaction.Name, MetaHash: hex.EncodeToString(b.Transaction.MetafileHash), Size: b.Transaction.Size}
			}(&chosenBlock)
		}

	}
}

// check if tx block is valid, i.e. there's no other committed block with the same name and its history is known
func (blockchainHandler *BlockchainHandler) isTxBlockValid(b BlockPublish) bool {

	isValid := true

	// check for same name
	blockchainHandler.committedHistory.Range(func(key interface{}, value interface{}) bool {
		block := value.(BlockPublish)

		if block.Transaction.Name == b.Transaction.Name {
			isValid = false
			return false
		}

		return true
	})

	// check for history validity
	if isValid {
		blockHash := b.PrevHash
		for blockHash != [32]byte{} {
			value, loaded := blockchainHandler.committedHistory.Load(blockHash)

			if !loaded {
				isValid = false
				break
			}

			block := value.(BlockPublish)
			blockHash = block.PrevHash
		}
	}

	return isValid
}

// check if consensus reached based on confirmations from previous rounds
func (blockchainHandler *BlockchainHandler) checkIfConsensusReached(confirmationsRoundS, confirmationsRoundS1 map[string]*TLCMessage, initialRound uint32) *TLCMessage {

	// get best from first confirmations of first round
	best := &TLCMessage{Fitness: 0}
	for _, message := range confirmationsRoundS {
		if message.Fitness > best.Fitness {
			best = message
		}
	}

	value, _ := blockchainHandler.messageSeen.Load(initialRound)
	tlcMap := value.(*SafeTLCMessagesMap)

	tlcMap.Mutex.RLock()
	messageSeen := tlcMap.OriginMessage
	tlcMap.Mutex.RUnlock()

	// check if higher than all messages seen from first round
	for _, saw := range messageSeen {
		if saw.Fitness >= best.Fitness && saw.TxBlock.Hash() != best.TxBlock.Hash() {
			return nil
		}
	}

	// check if block was reconfirmed
	for _, m1 := range confirmationsRoundS1 {
		if m1.TxBlock.Hash() == best.TxBlock.Hash() {
			return best
		}
	}

	return nil

	// ALTERNATIVE VERSION, DONT KNOW IF CORRECT OR NOT

	// // first condition: m belongs to confirmationsRoundS, i.e. originated from s and was witnessed by round s+1
	// for _, message := range confirmationsRoundS {

	// 	condition := false

	// 	// second condition: m witnessed by majority by round s+2
	// 	for _, m := range confirmationsRoundS1 {
	// 		// if m.TxBlock.Transaction.Name == message.TxBlock.Transaction.Name &&
	// 		// 	m.TxBlock.Transaction.Size == message.TxBlock.Transaction.Size &&
	// 		// 	string(m.TxBlock.Transaction.MetafileHash) ==hex.EncodeToStringmessage.TxBlock.Transaction.MetafileHash) &&
	// 		// 	m.TxBlock.PrevHash == message.TxBlock.PrevHash &&
	// 		if m.Fitness == message.Fitness {

	// 			condition = true
	// 			break
	// 		}
	// 	}

	// 	if !condition {
	// 		continue
	// 	}

	// 	for _, m0 := range confirmationsRoundS {
	// 		if m0.Fitness > message.Fitness {
	// 			condition = false
	// 			break
	// 		}
	// 	}

	// 	if !condition {
	// 		continue
	// 	}

	// 	for _, m1 := range confirmationsRoundS1 {
	// 		if m1.Fitness > message.Fitness {
	// 			condition = false
	// 			break
	// 		}
	// 	}

	// 	if condition {
	// 		return message
	// 	}
	// }

	// return nil
}
