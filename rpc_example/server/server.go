package main

import (
	"log"
	"net"
	"net/http"
	"net/rpc"

	"github.com/hashicorp/raft-boltdb"
	"github.com/komuw/kshaka"
)

func main() {
	// Create a store that will be used.
	// Ideally it should be a disk persisted store.
	// Any that implements hashicorp/raft StableStore
	// interface will suffice
	boltStore, err := raftboltdb.NewBoltStore("/tmp/bolt.db")
	if err != nil {
		panic(err)
	}

	// Create a Node with a list of additional nodes.
	// Number of nodes needed for quorom ought to be >= 3.

	// Note that in this example; nodes are using the same store
	// and are located in the same server/machine.
	// In practice however, nodes ideally should be
	// in different machines each with its own store.
	node1 := kshaka.NewNode(1, boltStore)
	node2 := kshaka.NewNode(2, boltStore)
	node3 := kshaka.NewNode(3, boltStore)

	// TODO: use rpc transport
	transport := &kshaka.InmemTransport{Node: node1}

	node1.AddTransport(transport)
	node2.AddTransport(transport)
	node3.AddTransport(transport)

	kshaka.MingleNodes(node1, node2, node3)

	////
	rpc.Register(node1)
	rpc.HandleHTTP()
	l, e := net.Listen("tcp", ":12000")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	http.Serve(l, nil)
}
