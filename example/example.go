package main

import (
	"fmt"

	"github.com/hashicorp/raft-boltdb"
	"github.com/komuw/kshaka"
)

func main() {

	fmt.Println("1")
	// create a store that will be used. Ideally it should be a disk persisted store.
	// any store that implements hashicorp/raft StableStore interface will suffice
	boltStore, err := raftboltdb.NewBoltStore("/tmp/bolt.db")
	if err != nil {
		panic(err)
	}
	fmt.Println("2")

	// The function that will be applied by CASPaxos. This will be applied to the
	// current value stored under the key passed into the Propose method of the proposer.
	var setFunc = func(val []byte) kshaka.ChangeFunction {
		return func(current []byte) ([]byte, error) {
			return val, nil
		}
	}
	fmt.Println("3")

	// create a Node with a list of additional nodes.
	// number of nodes needed for quorom ought to be >= 3
	node1 := kshaka.NewNode(boltStore)
	node2 := kshaka.NewNode(boltStore)
	node3 := kshaka.NewNode(boltStore)
	node4 := kshaka.NewNode(boltStore)

	fmt.Println("4")
	n := kshaka.NewNode(boltStore, node1, node2, node3, node4)

	key := []byte("name")
	val := []byte("Masta-Ace")

	fmt.Println("5")
	// make a proposition;
	// consensus via CASPaxos will happen and you will get the new state and any error back.
	newstate, err := kshaka.Propose(n, key, setFunc(val))
	if err != nil {
		fmt.Printf("err: %v", err)
	}
	fmt.Printf("newstate: %v", newstate)
	fmt.Println("6")
}
