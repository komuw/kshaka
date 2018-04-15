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
	transport1 := &kshaka.NetworkTransport{NodeAddrress: "127.0.0.1", NodePort: "15001"}
	transport2 := &kshaka.NetworkTransport{NodeAddrress: "127.0.0.1", NodePort: "15002"}
	transport3 := &kshaka.NetworkTransport{NodeAddrress: "127.0.0.1", NodePort: "15003"}

	node1.AddTransport(transport1)
	node2.AddTransport(transport2)
	node3.AddTransport(transport3)

	kshaka.MingleNodes(node1, node2, node3)

	////
	// rpc.HandleHTTP()
	handler1 := rpc.NewServer()
	handler1.Register(node1)
	handler1.HandleHTTP("/_goRPC1_", "/debug/rpc1")
	l, e := net.Listen("tcp", ":15001")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
	///
	handler2 := rpc.NewServer()
	handler2.Register(node2)
	handler2.HandleHTTP("/_goRPC2_", "/debug/rpc2")
	l2, e := net.Listen("tcp", ":15002")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l2, nil)
	///
	handler3 := rpc.NewServer()
	handler3.Register(node3)
	handler3.HandleHTTP("/_goRPC3_", "/debug/rpc3")
	l3, e := net.Listen("tcp", ":15003")
	if e != nil {
		log.Fatal("listen error:", e)
	}
	http.Serve(l3, nil)
}
