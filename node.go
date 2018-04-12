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
	"github.com/mattn/go-tty"
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
		// NB: you can call Propose on any of the nodes
		newstate, err := node2.Propose(key, setFunc(val))
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
	"os"
	"sync"

	"github.com/pkg/errors"
)

// Node satisfies the ProposerAcceptor interface.
// A Node is both a proposer and an acceptor. Most people will be interacting with a Node instead of a Proposer/Acceptor
type Node struct {
	ID       uint64
	Metadata map[string]string
	ballot   ballot
	nodes    []*Node

	// In general the "prepare" and "accept" operations affecting the same key should be mutually exclusive.
	// How to achieve this is an implementation detail.
	// eg in Gryadka it doesn't matter because the operations are implemented as Redis's stored procedures and Redis is single threaded. - Denis Rystsov
	// the mux protects the state(acceptorStore)
	sync.Mutex

	// acceptorStore is a StableStore implementation for durable state
	// It provides stable storage for many fields in raftState
	acceptorStore StableStore

	// TODO: maybe add a transport interface
	// so that lib users can roll their own
	Trans Transport
}

// NewNode creates a new node.
func NewNode(ID uint64, store StableStore, nodes ...*Node) *Node {
	n := &Node{ID: ID, acceptorStore: store}
	return n
}

// MingleNodes lets each node know about the other, including itself.
func MingleNodes(nodes ...*Node) {
	for _, n := range nodes {
		// guard against adding same node twice
		// TODO: fix this since it breaks down if len(nodes) > n.nodes even when some of those nodes already exist in n.nodes
		if len(n.nodes) < len(nodes) {
			n.nodes = append(n.nodes, nodes...)
		}
	}
}

// Propose is the method that clients call when they want to submit
// the f change function to a proposer.
// It takes the key whose value you want to apply the ChangeFunction to
// and also the ChangeFunction that will be applied to the value(contents) of that key.
func (n *Node) propose(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	// prepare phase
	currentState, err := n.sendPrepare(key)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		return nil, err
	}

	// accept phase
	newState, err := n.sendAccept(key, currentState, changeFunc)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		return nil, err
	}
	return newState, nil
}

// AddTransport adds transport to a node.
func (n *Node) AddTransport(t Transport) {
	n.Trans = t
}

// AddMetadata adds metadata to a node. eg name=myNode, env=production
func (n *Node) AddMetadata(metadata map[string]string) {
	n.Metadata = metadata
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
		return nil, errors.New(fmt.Sprintf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors))
	}
	if bytes.Equal(key, acceptedBallotKey(key)) {
		return nil, errors.New(fmt.Sprintf("the key:%v is reserved for storing kshaka internal state. chose another key.", acceptedBallotKey(key)))
	}

	p.incBallot()
	type prepareResult struct {
		acceptedState acceptorState
		err           error
	}

	prepareResultChan := make(chan prepareResult, noAcceptors)
	for _, a := range p.nodes {
		go func(a *Node) {
			acceptedState, err := a.Trans.TransportPrepare(p.ballot, key) // a.prepare(p.ballot, key)
			prepareResultChan <- prepareResult{acceptedState, err}
		}(a)
	}

	for i := 0; i < cap(prepareResultChan) && confirmationsNeeded > 0; i++ {
		res := <-prepareResultChan
		if res.err != nil {
			// conflict occured
			// TODO: make the io.writer configurable
			fmt.Fprintf(os.Stdout, "error:%+v", res.err)

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
		return nil, errors.New(fmt.Sprintf("confirmations:%v is less than required minimum of:%v", numberConfirmations, confirmationsNeeded))
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
		return nil, errors.New(fmt.Sprintf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors))
	}
	if bytes.Equal(key, acceptedBallotKey(key)) {
		return nil, errors.New(fmt.Sprintf("the key:%v is reserved for storing kshaka internal state. chose another key.", acceptedBallotKey(key)))
	}

	fmt.Println("ChangeFunc:::", changeFunc)
	newState, err := changeFunc(currentState)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to apply the ChangeFunction to value at key:%v", key))
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
			acceptedState, err := a.Trans.TransportAccept(p.ballot, key, newState) // a.accept(p.ballot, key, newState)
			acceptResultChan <- acceptResult{acceptedState, err}
		}(a)
	}

	for i := 0; i < cap(acceptResultChan) && confirmationsNeeded > 0; i++ {
		res := <-acceptResultChan
		if res.err != nil {
			// conflict occured
			fmt.Fprintf(os.Stdout, "error:%+v", res.err)
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
		return nil, errors.New(fmt.Sprintf("confirmations:%v is less than required minimum of:%v", numberConfirmations, confirmationsNeeded))
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
	if err != nil && err.Error() == "not found" {
		// see: issues/10
		// TODO: do better
		state, err = nil, nil
	}
	if err != nil {
		return acceptorState{}, errors.Wrap(err, fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, a.ID))
	}

	acceptedBallotBytes, err := a.acceptorStore.Get(acceptedBallotKey(key))
	if err != nil && err.Error() == "not found" {
		// unfortunate way of handling errors
		// TODO: do better
		acceptedBallotBytes, err = nil, nil
	}
	if err != nil {
		return acceptorState{state: state}, errors.Wrap(err, fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.ID))
	}
	var acceptedBallot ballot
	if !bytes.Equal(acceptedBallotBytes, nil) {
		// ie we found an accepted ballot
		acceptedBallotReader := bytes.NewReader(acceptedBallotBytes)
		dec := gob.NewDecoder(acceptedBallotReader)
		err = dec.Decode(&acceptedBallot)
		if err != nil {
			return acceptorState{state: state}, errors.Wrap(err, fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if acceptedBallot.Counter > b.Counter {
			return acceptorState{acceptedBallot: acceptedBallot, state: state}, errors.New(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.ID))
		}
	}

	promisedBallotBytes, err := a.acceptorStore.Get(promisedBallotKey(key))
	if err != nil && err.Error() == "not found" {
		// unfortunate way of handling errors
		// TODO: do better
		promisedBallotBytes, err = nil, nil
	}
	if err != nil {
		return acceptorState{state: state, acceptedBallot: acceptedBallot}, errors.Wrap(err, fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.ID))
	}
	var promisedBallot ballot
	if !bytes.Equal(promisedBallotBytes, nil) {
		// ie we found an promised ballot
		promisedBallotReader := bytes.NewReader(promisedBallotBytes)
		dec := gob.NewDecoder(promisedBallotReader)
		err = dec.Decode(&promisedBallot)
		if err != nil {
			return acceptorState{state: state, acceptedBallot: acceptedBallot}, errors.Wrap(err, fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if promisedBallot.Counter > b.Counter {
			return acceptorState{promisedBallot: promisedBallot, acceptedBallot: acceptedBallot, state: state}, errors.New(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, promisedBallot, a.ID))
		}
	}

	// TODO: this should be flushed to disk
	var ballotBuffer bytes.Buffer
	enc := gob.NewEncoder(&ballotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: promisedBallot}, errors.Wrap(err, fmt.Sprintf("unable to encode ballot:%v", b))
	}

	err = a.acceptorStore.Set(promisedBallotKey(key), ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: promisedBallot}, errors.Wrap(err, fmt.Sprintf("unable to flush ballot:%v to disk", b))
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
	if err != nil && err.Error() == "not found" {
		// unfortunate way of handling errors
		// TODO: do better
		state, err = nil, nil
	}
	if err != nil {
		return acceptorState{}, errors.Wrap(err, fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, a.ID))
	}

	acceptedBallotBytes, err := a.acceptorStore.Get(acceptedBallotKey(key))
	if err != nil && err.Error() == "not found" {
		// unfortunate way of handling errors
		// TODO: do better
		acceptedBallotBytes, err = nil, nil
	}
	if err != nil {
		return acceptorState{state: state}, errors.Wrap(err, fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.ID))
	}

	var acceptedBallot ballot
	if !bytes.Equal(acceptedBallotBytes, nil) {
		// ie we found an accepted ballot
		acceptedBallotReader := bytes.NewReader(acceptedBallotBytes)
		dec := gob.NewDecoder(acceptedBallotReader)
		err = dec.Decode(&acceptedBallot)
		if err != nil {
			return acceptorState{state: state}, errors.Wrap(err, fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if acceptedBallot.Counter > b.Counter {
			return acceptorState{acceptedBallot: acceptedBallot, state: state}, errors.New(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.ID))
		}
	}

	promisedBallotBytes, err := a.acceptorStore.Get(promisedBallotKey(key))
	if err != nil && err.Error() == "not found" {
		// unfortunate way of handling errors
		// TODO: do better
		promisedBallotBytes, err = nil, nil
	}
	if err != nil {
		return acceptorState{state: state, acceptedBallot: acceptedBallot}, errors.Wrap(err, fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.ID))
	}
	var promisedBallot ballot
	if !bytes.Equal(promisedBallotBytes, nil) {
		// ie we found an promised ballot
		promisedBallotReader := bytes.NewReader(promisedBallotBytes)
		dec := gob.NewDecoder(promisedBallotReader)
		err = dec.Decode(&promisedBallot)
		if err != nil {
			return acceptorState{state: state, acceptedBallot: acceptedBallot}, errors.Wrap(err, fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if promisedBallot.Counter > b.Counter {
			return acceptorState{promisedBallot: promisedBallot, acceptedBallot: acceptedBallot, state: state}, errors.New(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, promisedBallot, a.ID))
		}
	}

	// erase promised ballot
	err = a.acceptorStore.Set(promisedBallotKey(key), nil)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: promisedBallot}, errors.Wrap(err, fmt.Sprintf("unable to erase ballot:%v", b))
	}

	var ballotBuffer bytes.Buffer
	enc := gob.NewEncoder(&ballotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, errors.Wrap(err, fmt.Sprintf("unable to encode ballot:%v", b))
	}
	// TODO. NB: it is possible, from the following logic, for an acceptor to accept a ballot
	// but not accept the new state/value. ie if the call to acceptorStore.Set(acceptedBallotKey, ballotBuffer.Bytes()) succeeds
	// but acceptorStore.Set(key, state) fails.
	// we should think about the ramifications of that for a second.
	err = a.acceptorStore.Set(acceptedBallotKey(key), ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, errors.Wrap(err, fmt.Sprintf("unable to flush ballot:%v to disk", b))
	}

	err = a.acceptorStore.Set(key, state)
	if err != nil {
		return acceptorState{acceptedBallot: b, state: state}, errors.Wrap(err, fmt.Sprintf("unable to flush the new state:%v to disk", state))
	}

	return acceptorState{acceptedBallot: b, state: state}, nil

}
