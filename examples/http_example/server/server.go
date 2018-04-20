package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/hashicorp/raft-boltdb"
	"github.com/komuw/kshaka"
	"github.com/komuw/kshaka/httpTransport"
)

// HTTPtransportProposeRequest is the request sent as a proposal
// specifically for the HTTPtransport
type HTTPtransportProposeRequest struct {
	Key          []byte
	Val          []byte
	FunctionName string
}

var setFunc = func(val []byte) kshaka.ChangeFunction {
	return func(current []byte) ([]byte, error) {
		return val, nil
	}
}

var readFunc kshaka.ChangeFunction = func(current []byte) ([]byte, error) {
	return current, nil
}

func proposeHandler(n *kshaka.Node) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		proposeRequest := HTTPtransportProposeRequest{}
		err = json.Unmarshal(body, &proposeRequest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		changeFunc := readFunc
		if proposeRequest.FunctionName == "setFunc" {
			changeFunc = setFunc(proposeRequest.Val)
		}
		newState, err := n.Propose(proposeRequest.Key, changeFunc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		_, err = w.Write(newState)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func prepareHandler(n *kshaka.Node) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		prepareRequest := httpTransport.PrepareRequest{}
		err = json.Unmarshal(body, &prepareRequest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		aState, err := n.Prepare(prepareRequest.B, prepareRequest.Key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		acceptedState, err := json.Marshal(aState)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(acceptedState)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func acceptHandler(n *kshaka.Node) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		acceptRequest := httpTransport.AcceptRequest{}
		err = json.Unmarshal(body, &acceptRequest)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		aState, err := n.Accept(acceptRequest.B, acceptRequest.Key, acceptRequest.State)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		acceptedState, err := json.Marshal(aState)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(acceptedState)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func main() {
	// Create a store that will be used.
	// Ideally it should be a disk persisted store.
	// Any that implements hashicorp/raft StableStore interface will suffice
	boltStore1, err := raftboltdb.NewBoltStore("/tmp/bolt1.db")
	if err != nil {
		panic(err)
	}
	boltStore2, err := raftboltdb.NewBoltStore("/tmp/bolt2.db")
	if err != nil {
		panic(err)
	}
	boltStore3, err := raftboltdb.NewBoltStore("/tmp/bolt3.db")
	if err != nil {
		panic(err)
	}

	// Note that in this example; nodes are located in the same server/machine.
	// In practice however, nodes ideally should be in different machines
	node1 := kshaka.NewNode(1, boltStore1)
	node2 := kshaka.NewNode(2, boltStore2)
	node3 := kshaka.NewNode(3, boltStore3)

	transport1 := &httpTransport.HTTPtransport{
		NodeAddrress: "127.0.0.1",
		NodePort:     "15001",
		ProposeURI:   "/propose",
		PrepareURI:   "/prepare",
		AcceptURI:    "/accept"}
	transport2 := &httpTransport.HTTPtransport{
		NodeAddrress: "127.0.0.1",
		NodePort:     "15002",
		ProposeURI:   "/propose",
		PrepareURI:   "/prepare",
		AcceptURI:    "/accept"}
	transport3 := &httpTransport.HTTPtransport{
		NodeAddrress: "127.0.0.1",
		NodePort:     "15003",
		ProposeURI:   "/propose",
		PrepareURI:   "/prepare",
		AcceptURI:    "/accept"}

	node1.AddTransport(transport1)
	node2.AddTransport(transport2)
	node3.AddTransport(transport3)

	kshaka.MingleNodes(node1, node2, node3)

	http.HandleFunc("/propose", proposeHandler(node1))
	http.HandleFunc("/prepare", prepareHandler(node1))
	http.HandleFunc("/accept", acceptHandler(node1))

	go func() {
		log.Fatal(http.ListenAndServe(":15001", nil))
	}()

	go func() {
		log.Fatal(http.ListenAndServe(":15002", nil))
	}()

	log.Fatal(http.ListenAndServe(":15003", nil))
}
