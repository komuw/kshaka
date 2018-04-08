package main

import (
	"fmt"

	"github.com/hashicorp/raft-boltdb"
	"github.com/komuw/kshaka"
)

func main() {
	// create a store that will be used. Ideally it should be a disk persisted store.
	// any store that implements hashicorp/raft StableStore interface will suffice
	boltStore, err := raftboltdb.NewBoltStore("/tmp/bolt.db")
	if err != nil {
		panic(err)
	}

	// The function that will be applied by CASPaxos. This will be applied to the
	// current value stored under the key passed into the Propose method of the proposer.
	var setFunc = func(val []byte) kshaka.ChangeFunction {
		return func(current []byte) ([]byte, error) {
			return val, nil
		}
	}

	//create a Node with a list of additional nodes.
	// number of nodes needed for quorom ought to be >= 3.

	// Note that in this example; nodes are using the same store and are located in
	// the same server/machine.
	// In practice however, nodes ideally should be in different machines each with its own store.
	node1 := kshaka.NewNode(1, boltStore)
	node2 := kshaka.NewNode(2, boltStore)

	node3 := kshaka.NewNode(3, boltStore, node1, node2)

	key := []byte("name")
	val := []byte("Masta-Ace")

	// make a proposition;
	// consensus via CASPaxos will happen and you will get the new state and any error back.
	newstate, err := kshaka.Propose(node3, key, setFunc(val))
	if err != nil {
		fmt.Printf("err: %v", err)
	}
	fmt.Printf("newstate: %v", newstate)
}
