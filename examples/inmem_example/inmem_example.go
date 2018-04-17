package main

import (
	"fmt"

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

	// The function that will be applied by CASPaxos.
	// This will be applied to the current value stored
	// under the key passed into the Propose method of the proposer.
	var setFunc = func(val []byte) kshaka.ChangeFunction {
		return func(current []byte) ([]byte, error) {
			return val, nil
		}
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

	transport1 := &kshaka.InmemTransport{Node: node1}
	transport2 := &kshaka.InmemTransport{Node: node1}
	transport3 := &kshaka.InmemTransport{Node: node1}
	node1.AddTransport(transport1)
	node2.AddTransport(transport2)
	node3.AddTransport(transport3)

	kshaka.MingleNodes(node1, node2, node3)

	key := []byte("name")
	val := []byte("Masta-Ace")

	// make a proposition; consensus via CASPaxos will
	// happen and you will get the new state and any error back.
	// NB: you can call Propose on any of the nodes
	newstate, err := node2.Trans.TransportPropose(key, setFunc(val))
	if err != nil {
		fmt.Printf("err: %v", err)
	}
	fmt.Printf("\n newstate: %v \n", newstate)
}
