package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/hashicorp/raft-boltdb"
	"github.com/komuw/kshaka"
)

type TransportProposeRequest struct {
	Key []byte
	Val []byte
}

func proposeHandler(n *kshaka.Node) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("cool")
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("\n err: %+v \n", err)
			return
		}
		proposeRequest := TransportProposeRequest{}
		err = json.Unmarshal(body, &proposeRequest)
		if err != nil {
			fmt.Printf("\n err: %+v \n", err)
			return
		}
		fmt.Println("propose request body::", string(body))
		fmt.Printf("\n proposeRequest: %+v %+v  \n", string(proposeRequest.Key), string(proposeRequest.Val))
		fmt.Fprintf(w, "propose handler %s!", r.URL.Path[1:])
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

	// Create a Node with a list of additional nodes.
	// Number of nodes needed for quorom ought to be >= 3.

	// Note that in this example; nodes are located in the same server/machine.
	// In practice however, nodes ideally should be in different machines
	node1 := kshaka.NewNode(1, boltStore1)
	node2 := kshaka.NewNode(2, boltStore2)
	node3 := kshaka.NewNode(3, boltStore3)

	transport1 := &kshaka.HttpTransport{NodeAddrress: "127.0.0.1", NodePort: "15001"}
	transport2 := &kshaka.HttpTransport{NodeAddrress: "127.0.0.1", NodePort: "15002"}
	transport3 := &kshaka.HttpTransport{NodeAddrress: "127.0.0.1", NodePort: "15003"}

	node1.AddTransport(transport1)
	node2.AddTransport(transport2)
	node3.AddTransport(transport3)

	kshaka.MingleNodes(node1, node2, node3)

	////

	http.HandleFunc("/propose", proposeHandler(node1))
	go func() {
		log.Fatal(http.ListenAndServe(":15001", nil))
	}()

	go func() {
		log.Fatal(http.ListenAndServe(":15002", nil))
	}()

	log.Fatal(http.ListenAndServe(":15003", nil))
}
