# Kshaka
[![CircleCI](https://circleci.com/gh/komuw/kshaka.svg?style=svg)](https://circleci.com/gh/komuw/kshaka)


Kshaka is a Go implementation of the [CASPaxos](https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf) consensus protocol.                              
It's name is derived from the Kenyan hip hop group, Kalamashaka.                
>CASPaxos is a replicated state machine (RSM) protocol. Unlike Raft and Multi-Paxos, it doesn’t use leader election and log replication, thus avoiding associated complexity.                   
Its symmetric peer-to-peer approach achieves optimal commit latency in wide-area networks and doesn’t cause transient unavailability when any [N−1] of N nodes crash." - [The CASPaxos whitepaper](https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf)             

This is **work in progress, do not use it anywhere you would regret. API will change over time.**


# Installation
- todo          


# Usage
```go
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

	kshaka.MingleNodes(node1, node2, node3)

	key := []byte("name")
	val := []byte("Masta-Ace")

	// make a proposition; consensus via CASPaxos will
	// happen and you will get the new state and any error back.
	// NB: you can use any of the nodes as arg to Propose func
	newstate, err := kshaka.Propose(node3, key, setFunc(val))
	if err != nil {
		fmt.Printf("err: %v", err)
	}
	fmt.Printf("newstate: %v", newstate)
}

```               


# System design

### 1. Intro:           
- Clients initiate a request by communicating with a proposer; clients may be stateless, the system may have arbitrary numbers of clients.               
- Proposers perform the initialization by communicating with acceptors. 
Proposers keep minimal state needed to generate unique increasing update IDs (ballot numbers), the system may have arbitrary numbers of proposers.        
- Acceptors store the accepted value; the system should have 2F+1 acceptors to tolerate F failures.


- It’s convenient to use tuples as ballot numbers. 
To generate it a proposer combines its numerical ID with a local increasing counter: (counter, ID). 
To compare ballot tuples, we should compare the first component of the tuples and use ID only as a tiebreaker.
- When a proposer receives a conflicting message from an acceptor, it should fast-forward its counter to avoid a conflict in the future. 
If an acceptor returns a conflict if it already saw a greater ballot number during the prepare message, does the Proposer retry with a higher ballot number or does it just stop?
Ans: It doesn't matter from the protocol's point of view and different implementations may implement it in different ways. - https://twitter.com/rystsov/status/971797758284677120       
Proposers in Kshaka will, for the time been, will not retry after conflicts.

- Clients change its value by submitting side-effect free functions which take the current state as an argument and yield new as a result. 
Out of the concurrent requests only one can succeed;  we should acquire a lock:: https://github.com/gryadka/js/blob/dfc6ed6f7580c895a9db44d06756a3dd637e47f6/core/src/Proposer.js#L47-L48 

### 2. Algo:            

A. Prepare phase
- A client submits the f change function to a proposer.
- The proposer generates a ballot number, B, and sends ”prepare” messages containing that number(and it's ID) to the acceptors.
- Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
Persists the ballot number as a promise and returns a confirmation either with an empty value (if it hasn’t accepted any value yet) or with a tuple of an accepted value and its ballot number.
- Proposer waits for the F + 1 confirmations.               

B. Accept phase
- If they(prepare replies from acceptors) all contain the empty value, then the proposer defines the current state as ∅ otherwise it picks the value of the tuple with the highest ballot number.             
- Proposer applies the f function to the current state and sends the result, new state, along with the generated ballot number B (an ”accept” message) to the acceptors.
- Accept returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
Erases the promise, marks the received tuple (ballot number, value) as the accepted value and returns a confirmation        

C. End
- Proposer waits for the F + 1 confirmations.
- Proposer returns the new state to the client.       

### 3. Cluster membership change
- todo                 

### 4. Deleting record/s
- todo         

### 5. Optimizations
- todo          

# dev
debug one test;     
```
dlv test -- -test.v -test.run ^Test_proposer_Propose
```