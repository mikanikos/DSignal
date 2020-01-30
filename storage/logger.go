// Contributors: Dewmini Marakkalage

package storage

import (
	"encoding/hex"
	"fmt"
	"os"
)

func (ds *DStore) logMessage(format string, args ...interface{}) {
	ds.logMutex.Lock()
	defer ds.logMutex.Unlock()
	s := fmt.Sprintf("\nDStorage: ")
	s += fmt.Sprintf(format, args...)
	ds.displayLogs = append(ds.displayLogs, s)
	fmt.Fprintln(os.Stdout, s)
}

func (ds *DStore) logError(format string, args ...interface{}) {
	ds.logMutex.Lock()
	defer ds.logMutex.Unlock()
	fmt.Fprintf(os.Stderr, "DStorage: ")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}

func (ds *DStore) logDebug(format string, args ...interface{}) {
	ds.logMutex.Lock()
	defer ds.logMutex.Unlock()
	fmt.Fprintf(os.Stderr, "DStorage: ")
	fmt.Fprintf(os.Stderr, format, args...)
	fmt.Fprintln(os.Stderr)
}

func (ds *DStore) logPing(addr string, rpcid string) {
	ds.logError("Ping from %s rpcid %s", addr, rpcid)
}

func (ds *DStore) logNodeLookUp(query []byte, res []*RoutingEntry) {
	queryStr := hex.EncodeToString(query)
	resStr := ""
	for _, r := range res {
		resStr += " "
		resStr += hex.EncodeToString(r.ID)
		resStr += "[" + r.Address + "]"
	}
	ds.logMessage("NodeLookup query %s nodes%s", queryStr, resStr)
}
