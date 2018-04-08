/*
Package kshaka is a pure Go implementation of the CASPaxos consensus protocol.
It's name is derived from the Kenyan hip hop group, Kalamashaka.

"CASPaxos is a replicated state machine (RSM) protocol. Unlike Raft and Multi-Paxos,
it doesn't use leader election and log replication, thus avoiding associated complexity.
Its symmetric peer-to-peer approach achieves optimal commit latency in wide-area networks
and doesn't cause transient unavailability when any [N−1] of N nodes crash." - The CASPaxos whitepaper, https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf

Example usage:

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


TODO: add system design here.
*/
package kshaka

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
)

type prepareError string

func (e prepareError) Error() string {
	return string(e)
}

type acceptError string

func (e acceptError) Error() string {
	return string(e)
}

// Node satisfies the ProposerAcceptor interface.
// A Node is both a proposer and an acceptor. Most people will be interacting with a Node instead of a Proposer/Acceptor
type Node struct {
	ID     uint64
	ballot ballot
	nodes  []*Node

	// In general the "prepare" and "accept" operations affecting the same key should be mutually exclusive.
	// How to achieve this is an implementation detail.
	// eg in Gryadka it doesn't matter because the operations are implemented as Redis's stored procedures and Redis is single threaded. - Denis Rystsov
	// the mux protects the state(acceptorStore)
	sync.Mutex

	// acceptorStore is a StableStore implementation for durable state
	// It provides stable storage for many fields in raftState
	acceptorStore StableStore
}

// NewNode creates a new node. It also adds the created node to Node.nodes
// additionally if any nodes are supplied, they are also added to Node.nodes
func NewNode(ID uint64, store StableStore, nodes ...*Node) *Node {
	n := &Node{ID: ID, acceptorStore: store}
	n.nodes = []*Node{n}
	n.nodes = append(n.nodes, nodes...)
	return n
}

func (n *Node) addNode(node *Node) error {
	n.nodes = append(n.nodes, node)
	return nil
}

// monotonically increase the ballot
func (n *Node) incBallot() {
	n.ballot.Counter++
}

// The proposer generates a ballot number, B, and sends ”prepare” messages containing that number(and it's ID) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// If all replies from acceptors contain the empty value, then the proposer defines the current state as ∅
// otherwise it picks the value of the tuple with the highest ballot number.
func (p *Node) sendPrepare(key []byte) ([]byte, error) {
	var (
		noAcceptors         = len(p.nodes)
		F                   = (noAcceptors - 1) / 2 // number of failures we can tolerate
		confirmationsNeeded = F + 1
		highballotConfirm   ballot
		highballotConflict  = p.ballot
		currentState        []byte
		numberConflicts     int
		numberConfirmations int
	)

	if noAcceptors < minimumNoAcceptors {
		return nil, prepareError(fmt.Sprintf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors))
	}
	if bytes.Equal(key, acceptedBallotKey(key)) {
		return nil, prepareError(fmt.Sprintf("the key:%v is reserved for storing kshaka internal state. chose another key.", acceptedBallotKey(key)))
	}

	p.incBallot()
	type prepareResult struct {
		acceptedState acceptorState
		err           error
	}
	prepareResultChan := make(chan prepareResult, noAcceptors)
	for _, a := range p.nodes {
		go func(a *Node) {
			fmt.Printf("\n\n NODE: %#+v", a)
			acceptedState, err := a.prepare(p.ballot, key)
			prepareResultChan <- prepareResult{acceptedState, err}
		}(a)
	}

	for i := 0; i < cap(prepareResultChan) && confirmationsNeeded > 0; i++ {
		res := <-prepareResultChan
		if res.err != nil {
			// conflict occured
			numberConflicts++
			if res.acceptedState.acceptedBallot.Counter > highballotConflict.Counter {
				highballotConflict = res.acceptedState.acceptedBallot
			} else if res.acceptedState.promisedBallot.Counter > highballotConflict.Counter {
				highballotConflict = res.acceptedState.promisedBallot
			}
		} else {
			// confirmation occured.
			numberConfirmations++
			if res.acceptedState.acceptedBallot.Counter >= highballotConfirm.Counter {
				highballotConfirm = res.acceptedState.acceptedBallot
				currentState = res.acceptedState.state
			}
			confirmationsNeeded--
		}
	}

	// we didn't get F+1 confirmations
	if numberConfirmations < confirmationsNeeded {
		p.ballot.Counter = highballotConflict.Counter + 1
		return nil, prepareError(fmt.Sprintf("confirmations:%v is less than required minimum of:%v", numberConfirmations, confirmationsNeeded))
	}

	return currentState, nil
}

// Proposer applies the f function to the current state and sends the result, new state,
// along with the generated ballot number B (an ”accept” message) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// Proposer returns the new state to the client.
func (p *Node) sendAccept(key []byte, currentState []byte, changeFunc ChangeFunction) ([]byte, error) {

	/*
		Yes, acceptors should store tuple (promised ballot, accepted ballot and an accepted value) per key.
		Proposers, unlike acceptors, may use the same ballot number sequence.
		If we split a sequence of unique and increasing ballot numbers into several subsequences then any of them remains unique and increasing, so it's fine.
		- Rystsov
	*/
	var (
		noAcceptors         = len(p.nodes)
		F                   = (noAcceptors - 1) / 2 // number of failures we can tolerate
		confirmationsNeeded = F + 1
		highballotConflict  ballot
		numberConflicts     int
		numberConfirmations int
	)

	// probably we shouldn't call this method, sendAccept, if we havent called prepare yet and it is finished
	if noAcceptors < minimumNoAcceptors {
		return nil, acceptError(fmt.Sprintf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors))
	}
	if bytes.Equal(key, acceptedBallotKey(key)) {
		return nil, acceptError(fmt.Sprintf("the key:%v is reserved for storing kshaka internal state. chose another key.", acceptedBallotKey(key)))
	}

	newState, err := changeFunc(currentState)
	if err != nil {
		return nil, acceptError(fmt.Sprintf("%v", err))
	}
	// TODO: if newState == nil should we save it, or return error??
	// think about this some more

	type acceptResult struct {
		acceptedState acceptorState
		err           error
	}
	acceptResultChan := make(chan acceptResult, noAcceptors)
	for _, a := range p.nodes {
		go func(a *Node) {
			acceptedState, err := a.accept(p.ballot, key, newState)
			acceptResultChan <- acceptResult{acceptedState, err}
		}(a)
	}

	for i := 0; i < cap(acceptResultChan) && confirmationsNeeded > 0; i++ {
		res := <-acceptResultChan
		if res.err != nil {
			// conflict occured
			numberConflicts++
			if res.acceptedState.acceptedBallot.Counter > p.ballot.Counter {
				highballotConflict = res.acceptedState.acceptedBallot
			} else if res.acceptedState.promisedBallot.Counter > p.ballot.Counter {
				highballotConflict = res.acceptedState.promisedBallot
			}
		} else {
			// confirmation occured.
			numberConfirmations++
			confirmationsNeeded--
		}
	}

	// we didn't get F+1 confirmations
	if numberConfirmations < confirmationsNeeded {
		p.ballot.Counter = highballotConflict.Counter + 1
		return nil, acceptError(fmt.Sprintf("confirmations:%v is less than required minimum of:%v", numberConfirmations, confirmationsNeeded))
	}

	return newState, nil
}

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Persists the ballot number as a promise and returns a confirmation either with an empty value (if it hasn’t accepted any value yet)
// or with a tuple of an accepted value and its ballot number.
func (a *Node) prepare(b ballot, key []byte) (acceptorState, error) {
	// TODO: this locks are supposed to be per key
	// not method wide
	a.Lock()
	defer a.Unlock()

	state, err := a.acceptorStore.Get(key)
	if err != nil {
		// TODO: propagate errors!!
		fmt.Printf("\n\n unable to get state for key:%v from acceptor:%v, err:%v \n\n", key, a.ID, err)
		return acceptorState{}, prepareError(fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, a.ID))
	}
	fmt.Printf("\n\n found state:%v \n\n", state)

	acceptedBallotBytes, err := a.acceptorStore.Get(acceptedBallotKey(key))
	if err != nil {
		// TODO: propagate the underlying error
		return acceptorState{state: state}, prepareError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.ID))
	}
	var acceptedBallot ballot
	if !bytes.Equal(acceptedBallotBytes, nil) {
		// ie we found an accepted ballot
		acceptedBallotReader := bytes.NewReader(acceptedBallotBytes)
		dec := gob.NewDecoder(acceptedBallotReader)
		err = dec.Decode(&acceptedBallot)
		if err != nil {
			return acceptorState{state: state}, prepareError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if acceptedBallot.Counter > b.Counter {
			return acceptorState{acceptedBallot: acceptedBallot, state: state}, prepareError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.ID))
		}
	}

	promisedBallotBytes, err := a.acceptorStore.Get(promisedBallotKey(key))
	if err != nil {
		// TODO: propagate the underlying error
		return acceptorState{state: state, acceptedBallot: acceptedBallot}, prepareError(fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.ID))
	}
	var promisedBallot ballot
	if !bytes.Equal(promisedBallotBytes, nil) {
		// ie we found an promised ballot
		promisedBallotReader := bytes.NewReader(promisedBallotBytes)
		dec := gob.NewDecoder(promisedBallotReader)
		err = dec.Decode(&promisedBallot)
		if err != nil {
			return acceptorState{state: state, acceptedBallot: acceptedBallot}, prepareError(fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if promisedBallot.Counter > b.Counter {
			return acceptorState{promisedBallot: promisedBallot, acceptedBallot: acceptedBallot, state: state}, prepareError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, promisedBallot, a.ID))
		}
	}

	// TODO: this should be flushed to disk
	var ballotBuffer bytes.Buffer
	enc := gob.NewEncoder(&ballotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: promisedBallot}, prepareError(fmt.Sprintf("%v", err))
	}

	err = a.acceptorStore.Set(promisedBallotKey(key), ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: promisedBallot}, prepareError(fmt.Sprintf("%v", err))
	}
	return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: b}, nil
}

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Erases the promise, marks the received tuple (ballot number, value) as the accepted value and returns a confirmation
func (a *Node) accept(b ballot, key []byte, state []byte) (acceptorState, error) {
	/*
		Yes, acceptors should store tuple (promised ballot, accepted ballot and an accepted value) per key.
		Proposers, unlike acceptors, may use the same ballot number sequence.
		If we split a sequence of unique and increasing ballot numbers into several subsequences then any of them remains unique and increasing, so it's fine.
		- Rystsov
	*/

	// we still need to unlock even when using a StableStore as the store of state.
	// this is because, someone may provide us with non-concurrent safe StableStore
	a.Lock()
	defer a.Unlock()

	state, err := a.acceptorStore.Get(key)
	if err != nil {
		return acceptorState{}, acceptError(fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, a.ID))
	}

	acceptedBallotBytes, err := a.acceptorStore.Get(acceptedBallotKey(key))
	if err != nil {
		return acceptorState{state: state}, acceptError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.ID))
	}

	fmt.Printf("\n\n acceptedBallotBytes: %#+v", acceptedBallotBytes)

	var acceptedBallot ballot
	if !bytes.Equal(acceptedBallotBytes, nil) {
		// ie we found an accepted ballot
		acceptedBallotReader := bytes.NewReader(acceptedBallotBytes)
		dec := gob.NewDecoder(acceptedBallotReader)
		err = dec.Decode(&acceptedBallot)
		if err != nil {
			fmt.Printf("\n\n err: %#+v", err)
			return acceptorState{state: state}, acceptError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if acceptedBallot.Counter > b.Counter {
			return acceptorState{acceptedBallot: acceptedBallot, state: state}, acceptError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.ID))
		}
	}

	promisedBallotBytes, err := a.acceptorStore.Get(promisedBallotKey(key))
	if err != nil {
		// TODO: propagate the underlying error
		return acceptorState{state: state, acceptedBallot: acceptedBallot}, acceptError(fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.ID))
	}
	var promisedBallot ballot
	if !bytes.Equal(promisedBallotBytes, nil) {
		// ie we found an promised ballot
		promisedBallotReader := bytes.NewReader(promisedBallotBytes)
		dec := gob.NewDecoder(promisedBallotReader)
		err = dec.Decode(&promisedBallot)
		if err != nil {
			return acceptorState{state: state, acceptedBallot: acceptedBallot}, acceptError(fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if promisedBallot.Counter > b.Counter {
			return acceptorState{promisedBallot: promisedBallot, acceptedBallot: acceptedBallot, state: state}, acceptError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, promisedBallot, a.ID))
		}
	}

	// erase promised ballot
	err = a.acceptorStore.Set(promisedBallotKey(key), nil)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: promisedBallot}, acceptError(fmt.Sprintf("%v", err))
	}

	var ballotBuffer bytes.Buffer
	enc := gob.NewEncoder(&ballotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, acceptError(fmt.Sprintf("%v", err))
	}
	// TODO. NB: it is possible, from the following logic, for an acceptor to accept a ballot
	// but not accept the new state/value. ie if the call to acceptorStore.Set(acceptedBallotKey, ballotBuffer.Bytes()) succeeds
	// but acceptorStore.Set(key, state) fails.
	// we should think about the ramifications of that for a second.
	err = a.acceptorStore.Set(acceptedBallotKey(key), ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, acceptError(fmt.Sprintf("%v", err))
	}
	fmt.Printf("\n\n key: %#+v. state: %#+v", string(key), string(state))
	err = a.acceptorStore.Set(key, state)
	if err != nil {
		return acceptorState{acceptedBallot: b, state: state}, acceptError(fmt.Sprintf("%v", err))
	}

	return acceptorState{acceptedBallot: b, state: state}, nil

}
