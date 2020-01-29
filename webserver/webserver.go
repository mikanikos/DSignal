package webserver

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/mikanikos/DSignal/client/clientsender"
	"github.com/mikanikos/DSignal/gossiper"
	"github.com/mikanikos/DSignal/helpers"
	"github.com/mikanikos/DSignal/adssignal"
)

// Webserver struct
type Webserver struct {
	Gossiper *gossiper.Gossiper
	Signal 	 *adssignal.SignalHandler
	Client   *clientsender.Client
}

// NewWebserver for gui, has the gossiper instance to get values to display in the ui and a client to communicate values to the gossiper using the standard protocol
func NewWebserver(uiPort string, gossiper *gossiper.Gossiper, signal *adssignal.SignalHandler) *Webserver {
	return &Webserver{
		Gossiper: gossiper,
		Signal: signal,
		Client:   clientsender.NewClient(uiPort),
	}
}

// Run webserver to handle get and post requests
func (webserver *Webserver) Run(portGUI string) {

	r := mux.NewRouter()

	r.HandleFunc("/message", webserver.getMessageHandler).Methods("GET")
	r.HandleFunc("/message", webserver.postMessageHandler).Methods("POST")
	r.HandleFunc("/node", webserver.getNodeHandler).Methods("GET")
	r.HandleFunc("/node", webserver.postNodeHandler).Methods("POST")
	r.HandleFunc("/id", webserver.getIDHandler).Methods("GET")
	r.HandleFunc("/origin", webserver.getOriginHandler).Methods("GET")
	r.HandleFunc("/file", webserver.getFileHandler).Methods("GET")
	r.HandleFunc("/download", webserver.getDownloadHandler).Methods("GET")
	r.HandleFunc("/search", webserver.getSearchHandler).Methods("GET")
	r.HandleFunc("/round", webserver.getRoundHandler).Methods("GET")
	r.HandleFunc("/bcLogs", webserver.getBCLogsHandler).Methods("GET")
	r.HandleFunc("/blockchain", webserver.getBlockchainHandler).Methods("GET")
	r.HandleFunc("/identity", webserver.getIdentityHandler).Methods("get")
	r.HandleFunc("/signal", webserver.postSignalHandler).Methods("post")

	r.PathPrefix("/").Handler(http.StripPrefix("/", http.FileServer(http.Dir("./webserver"))))

	log.Fatal(http.ListenAndServe(":"+portGUI, r))
}

// function to write json data in the http header
func writeJSON(w http.ResponseWriter, payload interface{}) {
	bytes, err := json.Marshal(payload)
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}


// get and display signal log
func (webserver *Webserver) getIdentityHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, webserver.Signal.GetIdentity())
}


// get and display signal log
// send client message to gossiper with the arguments given
func (webserver *Webserver) postSignalHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}

	user := r.PostForm.Get("name")

	var payload = webserver.Signal.GetRatchetMessages(user)
	writeJSON(w, payload)	
}

// get and display full blockchain 
func (webserver *Webserver) getBlockchainHandler(w http.ResponseWriter, r *http.Request) {
	var payload = webserver.Gossiper.GetBlockchain()
	writeJSON(w, payload)
}

// get and display blockchain log messages
func (webserver *Webserver) getBCLogsHandler(w http.ResponseWriter, r *http.Request) {
	var payload = gossiper.GetBlockchainList(webserver.Gossiper.GetBlockchainLogs())
	writeJSON(w, payload)
}

// get and display current round for tlc
func (webserver *Webserver) getRoundHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, webserver.Gossiper.GetRound())
}

// get and display the latest files that have been found in a search
func (webserver *Webserver) getSearchHandler(w http.ResponseWriter, r *http.Request) {
	var payload = gossiper.GetFilesList(webserver.Gossiper.GetSearchedFiles())
	writeJSON(w, payload)
}

// get and display the downloaded files so far
func (webserver *Webserver) getDownloadHandler(w http.ResponseWriter, r *http.Request) {
	var payload = gossiper.GetFilesList(webserver.Gossiper.GetDownloadedFiles())
	writeJSON(w, payload)
}

// get and display the indexed files so far
func (webserver *Webserver) getFileHandler(w http.ResponseWriter, r *http.Request) {
	var payload = gossiper.GetFilesList(webserver.Gossiper.GetIndexedFiles())
	writeJSON(w, payload)
}

// get and display the latest gossip messages
func (webserver *Webserver) getMessageHandler(w http.ResponseWriter, r *http.Request) {
	var payload = gossiper.GetMessagesList(webserver.Gossiper.GetLatestRumorMessages())
	writeJSON(w, payload)
}

// send client message to gossiper with the arguments given
func (webserver *Webserver) postMessageHandler(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	helpers.ErrorCheck(err, false)
	if err != nil {
		return
	}

	// parse post message
	message := r.PostForm.Get("text")
	destination := r.PostForm.Get("destination")
	file := r.PostForm.Get("file")
	request := r.PostForm.Get("request")
	keywords := r.PostForm.Get("keywords")
	budget := r.PostForm.Get("budget")

	if budget == "" {
		budget = "0"
	}
	budgetValue, err := strconv.ParseUint(budget, 10, 64)
	helpers.ErrorCheck(err, false)

	// send message with parameters through client interface
	webserver.Client.SendMessage(message, &destination, &file, &request, keywords, budgetValue)
}

// get and display the peers (neighbour nodes) known by the gossiper
func (webserver *Webserver) getNodeHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, helpers.GetArrayStringFromAddresses(webserver.Gossiper.GetPeers()))
}

// add peer given by gui to the gossiper
func (webserver *Webserver) postNodeHandler(w http.ResponseWriter, r *http.Request) {
	bytes, err := ioutil.ReadAll(r.Body)
	peer := string(bytes)
	peerAddr, err := net.ResolveUDPAddr("udp4", peer)
	if err == nil {
		webserver.Gossiper.AddPeer(peerAddr)
	}
}

// get and display gossiper name
func (webserver *Webserver) getIDHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, webserver.Gossiper.GetName())
}

// get and display origin nodes names
func (webserver *Webserver) getOriginHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, webserver.Gossiper.GetOrigins())
}
