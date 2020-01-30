package gossiper

import (
	"github.com/mikanikos/DSignal/storage"
)

type DStorageHandler struct {
	DStore *storage.DStore
}

func NewDStorageHandler(name, addr string) *DStorageHandler {
	dStore := storage.NewDStore(name, addr, 3, 2) 
	return &DStorageHandler{ DStore: dStore }
}