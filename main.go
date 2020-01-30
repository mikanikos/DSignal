/* Authors: Andrea Piccione, Sergio Roldan */

package main

import (
	"flag"
	"fmt"
	"github.com/mikanikos/DSignal/adssignal"
	"github.com/mikanikos/DSignal/gossiper"
	"github.com/mikanikos/DSignal/helpers"
	"github.com/mikanikos/DSignal/webserver"
	"github.com/mikanikos/DSignal/whisper"
)

// main entry point of the DSignal app
func main() {

	// parsing arguments according to the specification given
	guiPort := flag.String("GUIPort", "", "port for the graphical interface")
	uiPort := flag.String("UIPort", "8080", "port for the command line interface")
	gossipAddr := flag.String("gossipAddr", "127.0.0.1:5000", "ip:port for the gossiper")
	gossipName := flag.String("name", "", "name of the gossiper")
	peers := flag.String("peers", "", "comma separated list of peers of the form ip:port")
	peersNumber := flag.Uint64("N", 1, "total number of peers in the network")
	simple := flag.Bool("simple", false, "run gossiper in simple broadcast mode")
	hw3ex2 := flag.Bool("hw3ex2", false, "enable gossiper mode for knowing the transactions from other peers")
	hw3ex3 := flag.Bool("hw3ex3", false, "enable gossiper mode for round based gossiping")
	hw3ex4 := flag.Bool("hw3ex4", false, "enable gossiper mode for consensus agreement")
	ackAll := flag.Bool("ackAll", false, "make gossiper ack all tlc messages regardless of the ID")
	signal := flag.Bool("signal", false, "run gossiper using double ratchet and x3dh")
	antiEntropy := flag.Uint("antiEntropy", 10, "timeout in seconds for anti-entropy")
	rtimer := flag.Uint("rtimer", 0, "timeout in seconds to send route rumors")
	hopLimit := flag.Uint("hopLimit", 10, "hop limit value (TTL) for a packet")
	stubbornTimeout := flag.Uint("stubbornTimeout", 5, "stubborn timeout to resend a txn BlockPublish until it receives a majority of acks")

	flag.Parse()

	// set flags that are used througout the application
	gossiper.SetAppConstants(*simple, *hw3ex2, *hw3ex3, *hw3ex4, *ackAll, *signal, *hopLimit, *stubbornTimeout, *rtimer, *antiEntropy)

	// create new gossiper instance
	g := gossiper.NewGossiper(*gossipName, *gossipAddr, helpers.BaseAddress+":"+*uiPort, *peers, *peersNumber)

	w := whisper.NewWhisper(g)
	
	s := adssignal.NewSignalHandler(*gossipName, g, w)

	// run gossiper
	g.Run(*gossipAddr)
	fmt.Println("\nGossiper running")

	w.Run()
	fmt.Println("\nWhisper running")

	s.Run()
	fmt.Println("\nSignal running")

	// if gui port specified, create and run the web server (if not, avoid waste of resources for performance reasons)
	if *guiPort != "" {
		ws := webserver.NewWebserver(*uiPort, g, s)
		go ws.Run(*guiPort)
	}

	// wait forever
	select {}
}
