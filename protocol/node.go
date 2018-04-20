/*
Package protocol is a pure Go implementation of the CASPaxos consensus protocol.
It's name is derived from the Kenyan hip hop group, Kalamashaka.

"CASPaxos is a replicated state machine (RSM) protocol. Unlike Raft and Multi-Paxos,
it doesn't use leader election and log replication, thus avoiding associated complexity.
Its symmetric peer-to-peer approach achieves optimal commit latency in wide-area networks
and doesn't cause transient unavailability when any [N−1] of N nodes crash." - The CASPaxos whitepaper, https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf

Example usage:

	package main

	import (
	"github.com/sanity-io/litter"
	"golang_org/x/net/lif"
	"github.com/sanity-io/litter"
	"golang_org/x/net/lif"
		"fmt"

		"github.com/hashicorp/raft-boltdb"
		"github.com/komuw/kshaka/protocol"
		"github.com/komuw/kshaka/transport"
	)

	func main() {
		// The store should, ideally be disk persisted.
		// Any that implements hashicorp/raft StableStore interface will suffice
		boltStore, err := raftboltdb.NewBoltStore("/tmp/bolt.db")
		if err != nil {
			panic(err)
		}

		// The function that will be applied by CASPaxos.
		// This will be applied to the current value stored
		// under the key passed into the Propose method of the proposer.
		var setFunc = func(val []byte) protocol.ChangeFunction {
			return func(current []byte) ([]byte, error) {
				return val, nil
			}
		}

		// Note that, in practice, nodes ideally should be
		// in different machines each with its own store.
		node1 := protocol.NewNode(1, boltStore)
		node2 := protocol.NewNode(2, boltStore)
		node3 := protocol.NewNode(3, boltStore)

		transport1 := &transport.InmemTransport{Node: node1}
		transport2 := &transport.InmemTransport{Node: node2}
		transport3 := &transport.InmemTransport{Node: node3}
		node1.AddTransport(transport1)
		node2.AddTransport(transport2)
		node3.AddTransport(transport3)

		protocol.MingleNodes(node1, node2, node3)

		key := []byte("name")
		val := []byte("Masta-Ace")

		// make a proposition; consensus via CASPaxos will
		newstate, err := node2.Propose(key, setFunc(val))
		if err != nil {
			fmt.Printf("err: %v", err)
		}
		fmt.Printf("\n newstate: %v \n", newstate)
	}

TODO: add system design here.
*/
package protocol

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	"github.com/sanity-io/litter"
)

const stableStoreNotFoundErr = "not found"

// Node satisfies the ProposerAcceptor interface.
// A Node is both a proposer and an acceptor. Most people will be interacting with a Node instead of a Proposer/Acceptor
type Node struct {
	// ID should be unique to each node in the cluster.
	ID       uint64
	Metadata map[string]string
	Ballot   Ballot
	nodes    []*Node

	// In general the "prepare" and "accept" operations affecting the same key should be mutually exclusive.
	// How to achieve this is an implementation detail.
	// eg in Gryadka it doesn't matter because the operations are implemented as Redis's stored procedures and Redis is single threaded. - Denis Rystsov
	// the mux protects the state(acceptorStore)
	sync.Mutex

	// acceptorStore is a StableStore implementation for durable state
	// It provides stable storage for many fields in raftState
	acceptorStore StableStore

	Trans Transport
}

// NewNode creates a new node.
func NewNode(ID uint64, store StableStore) *Node {
	n := &Node{ID: ID, acceptorStore: store}
	return n
}

func removeDuplicatesNodes(a []*Node) []*Node {
	result := []*Node{}
	seen := map[uint64]*Node{}
	for _, val := range a {
		if _, ok := seen[val.ID]; !ok {
			result = append(result, val)
			seen[val.ID] = val
		}
	}
	return result
}

// MingleNodes lets each node know about the other, including itself.
func MingleNodes(nodes ...*Node) {
	for _, n := range nodes {
		incomingNodes := nodes
		unDedupedNodes := append(incomingNodes, n.nodes...)
		dedupedNodes := removeDuplicatesNodes(unDedupedNodes)
		n.nodes = dedupedNodes
	}
}

// AddTransport adds transport to a node.
func (n *Node) AddTransport(t Transport) {
	n.Trans = t
}

// AddMetadata adds metadata to a node. eg name=myNode, env=production
func (n *Node) AddMetadata(metadata map[string]string) {
	n.Metadata = metadata
}

// monotonically increase the Ballot
func (n *Node) incBallot() {
	n.Ballot.Counter++
}

// Propose is the method that clients call when they want to submit
// the f change function to a proposer.
// It takes the key whose value you want to apply the ChangeFunction to
// and also the ChangeFunction that will be applied to the value(contents) of that key.
func (n *Node) Propose(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	// prepare phase
	currentState, err := n.sendPrepare(key)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		return nil, err
	}
	fmt.Printf("currentState: %+v %+v\n", currentState, string(currentState))

	// accept phase
	newState, err := n.sendAccept(key, currentState, changeFunc)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		return nil, err
	}
	fmt.Printf("newState: %+v %+v\n", newState, string(newState))

	return newState, nil
}

// The proposer generates a Ballot number, B, and sends ”prepare” messages containing that number(and it's ID) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// If all replies from acceptors contain the empty value, then the proposer defines the current state as ∅
// otherwise it picks the value of the tuple with the highest Ballot number.
func (n *Node) sendPrepare(key []byte) ([]byte, error) {
	var (
		noAcceptors         = len(n.nodes)
		F                   = (noAcceptors - 1) / 2 // number of failures we can tolerate
		confirmationsNeeded = F + 1
		highBallotConfirm   Ballot
		highBallotConflict  = n.Ballot
		currentState        []byte
		numberConflicts     int
		numberConfirmations int
	)

	if noAcceptors < minimumNoAcceptors {
		return nil, fmt.Errorf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors)
	}
	if bytes.Equal(key, acceptedBallotKey(key)) {
		return nil, fmt.Errorf("the key:%v is reserved for storing kshaka internal state. chose another key", acceptedBallotKey(key))
	}

	n.incBallot()
	type prepareResult struct {
		acceptedState AcceptorState
		err           error
	}

	prepareResultChan := make(chan prepareResult, noAcceptors)
	for _, a := range n.nodes {
		go func(a *Node) {
			litter.Dump(a.Trans)
			acceptedState, err := a.Trans.TransportPrepare(n.Ballot, key)
			prepareResultChan <- prepareResult{acceptedState, err}
		}(a)
	}

	for i := 0; i < cap(prepareResultChan) && confirmationsNeeded > 0; i++ {
		res := <-prepareResultChan
		if res.err != nil {
			// conflict occurred
			numberConflicts++
			if res.acceptedState.AcceptedBallot.Counter > highBallotConflict.Counter {
				highBallotConflict = res.acceptedState.AcceptedBallot
			} else if res.acceptedState.PromisedBallot.Counter > highBallotConflict.Counter {
				highBallotConflict = res.acceptedState.PromisedBallot
			}
		} else {
			// confirmation occurred.
			numberConfirmations++
			if res.acceptedState.AcceptedBallot.Counter >= highBallotConfirm.Counter {
				highBallotConfirm = res.acceptedState.AcceptedBallot
				currentState = res.acceptedState.State
			}
			confirmationsNeeded--
		}
	}

	// we didn't get F+1 confirmations
	if numberConfirmations < confirmationsNeeded {
		n.Ballot.Counter = highBallotConflict.Counter + 1
		return nil, fmt.Errorf("confirmations:%v is less than required minimum of:%v", numberConfirmations, confirmationsNeeded)
	}

	return currentState, nil
}

// Proposer applies the f function to the current state and sends the result, new state,
// along with the generated Ballot number B (an ”accept” message) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// Proposer returns the new state to the client.
func (n *Node) sendAccept(key []byte, currentState []byte, changeFunc ChangeFunction) ([]byte, error) {

	/*
		Yes, acceptors should store tuple (promised Ballot, accepted Ballot and an accepted value) per key.
		Proposers, unlike acceptors, may use the same Ballot number sequence.
		If we split a sequence of unique and increasing Ballot numbers into several subsequences then any of them remains unique and increasing, so it's fine.
		- Rystsov
	*/
	var (
		noAcceptors         = len(n.nodes)
		F                   = (noAcceptors - 1) / 2 // number of failures we can tolerate
		confirmationsNeeded = F + 1
		highBallotConflict  Ballot
		numberConflicts     int
		numberConfirmations int
	)

	// probably we shouldn't call this method, sendAccept, if we havent called prepare yet and it is finished
	if noAcceptors < minimumNoAcceptors {
		return nil, fmt.Errorf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors)
	}
	if bytes.Equal(key, acceptedBallotKey(key)) {
		return nil, fmt.Errorf("the key:%v is reserved for storing kshaka internal state. chose another key", acceptedBallotKey(key))
	}

	newState, err := changeFunc(currentState)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("unable to apply the ChangeFunction to value at key:%v", key))
	}
	// TODO: if newState == nil should we save it, or return error??
	// think about this some more

	type acceptResult struct {
		acceptedState AcceptorState
		err           error
	}
	acceptResultChan := make(chan acceptResult, noAcceptors)
	for _, a := range n.nodes {
		go func(a *Node) {
			acceptedState, err := a.Trans.TransportAccept(n.Ballot, key, newState)
			acceptResultChan <- acceptResult{acceptedState, err}
		}(a)
	}

	for i := 0; i < cap(acceptResultChan) && confirmationsNeeded > 0; i++ {
		res := <-acceptResultChan
		if res.err != nil {
			// conflict occurred
			numberConflicts++
			if res.acceptedState.AcceptedBallot.Counter > n.Ballot.Counter {
				highBallotConflict = res.acceptedState.AcceptedBallot
			} else if res.acceptedState.PromisedBallot.Counter > n.Ballot.Counter {
				highBallotConflict = res.acceptedState.PromisedBallot
			}
		} else {
			// confirmation occurred.
			numberConfirmations++
			confirmationsNeeded--
		}
	}

	// we didn't get F+1 confirmations
	if numberConfirmations < confirmationsNeeded {
		n.Ballot.Counter = highBallotConflict.Counter + 1
		return nil, fmt.Errorf("confirmations:%v is less than required minimum of:%v", numberConfirmations, confirmationsNeeded)
	}

	return newState, nil
}

// Prepare handles the prepare phase for an acceptor(node).
// An Acceptor returns a conflict if it already saw a greater Ballot number, it also submits the Ballot and accepted value it has.
// Persists the Ballot number as a promise and returns a confirmation either with an empty value (if it hasn’t accepted any value yet)
// or with a tuple of an accepted value and its Ballot number.
func (n *Node) Prepare(b Ballot, key []byte) (AcceptorState, error) {
	// TODO: this locks are supposed to be per key
	// not method wide
	n.Lock()
	defer n.Unlock()

	state, err := n.acceptorStore.Get(key)
	if err != nil && err.Error() == stableStoreNotFoundErr {
		// see: issues/10
		// TODO: do better
		state, err = nil, nil
	}
	if err != nil {
		return AcceptorState{}, errors.Wrap(err, fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, n.ID))
	}

	acceptedBallotBytes, err := n.acceptorStore.Get(acceptedBallotKey(key))
	if err != nil && err.Error() == stableStoreNotFoundErr {
		// unfortunate way of handling errors
		// TODO: do better
		acceptedBallotBytes, err = nil, nil
	}
	if err != nil {
		return AcceptorState{State: state}, errors.Wrap(err, fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", n.ID))
	}
	var acceptedBallot Ballot
	if !bytes.Equal(acceptedBallotBytes, nil) {
		// ie we found an accepted Ballot
		acceptedBallotReader := bytes.NewReader(acceptedBallotBytes)
		dec := gob.NewDecoder(acceptedBallotReader)
		err = dec.Decode(&acceptedBallot)
		if err != nil {
			return AcceptorState{State: state}, errors.Wrap(err, fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", n.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if acceptedBallot.Counter > b.Counter {
			return AcceptorState{AcceptedBallot: acceptedBallot, State: state}, fmt.Errorf("submitted Ballot:%v is less than Ballot:%v of acceptor:%v", b, acceptedBallot, n.ID)
		}
	}

	promisedBallotBytes, err := n.acceptorStore.Get(promisedBallotKey(key))
	if err != nil && err.Error() == stableStoreNotFoundErr {
		// unfortunate way of handling errors
		// TODO: do better
		promisedBallotBytes, err = nil, nil
	}
	if err != nil {
		return AcceptorState{State: state, AcceptedBallot: acceptedBallot}, errors.Wrap(err, fmt.Sprintf("unable to get promisedBallot of acceptor:%v", n.ID))
	}
	var promisedBallot Ballot
	if !bytes.Equal(promisedBallotBytes, nil) {
		// ie we found an promised Ballot
		promisedBallotReader := bytes.NewReader(promisedBallotBytes)
		dec := gob.NewDecoder(promisedBallotReader)
		err = dec.Decode(&promisedBallot)
		if err != nil {
			return AcceptorState{State: state, AcceptedBallot: acceptedBallot}, errors.Wrap(err, fmt.Sprintf("unable to get promisedBallot of acceptor:%v", n.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if promisedBallot.Counter > b.Counter {
			return AcceptorState{PromisedBallot: promisedBallot, AcceptedBallot: acceptedBallot, State: state}, fmt.Errorf("submitted Ballot:%v is less than Ballot:%v of acceptor:%v", b, promisedBallot, n.ID)
		}
	}

	// TODO: this should be flushed to disk
	var BallotBuffer bytes.Buffer
	enc := gob.NewEncoder(&BallotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return AcceptorState{AcceptedBallot: acceptedBallot, State: state, PromisedBallot: promisedBallot}, errors.Wrap(err, fmt.Sprintf("unable to encode Ballot:%v", b))
	}

	err = n.acceptorStore.Set(promisedBallotKey(key), BallotBuffer.Bytes())
	if err != nil {
		return AcceptorState{AcceptedBallot: acceptedBallot, State: state, PromisedBallot: promisedBallot}, errors.Wrap(err, fmt.Sprintf("unable to flush Ballot:%v to disk", b))
	}
	return AcceptorState{AcceptedBallot: acceptedBallot, State: state, PromisedBallot: b}, nil
}

// Accept handles the accept phase for an acceptor(node).
// An Acceptor returns a conflict if it already saw a greater Ballot number, it also submits the Ballot and accepted value it has.
// Erases the promise, marks the received tuple (Ballot number, value) as the accepted value and returns a confirmation
func (n *Node) Accept(b Ballot, key []byte, newState []byte) (AcceptorState, error) {
	/*
		Yes, acceptors should store tuple (promised Ballot, accepted Ballot and an accepted value) per key.
		Proposers, unlike acceptors, may use the same Ballot number sequence.
		If we split a sequence of unique and increasing Ballot numbers into several subsequences then any of them remains unique and increasing, so it's fine.
		- Rystsov
	*/

	// we still need to unlock even when using a StableStore as the store of state.
	// this is because, someone may provide us with non-concurrent safe StableStore
	n.Lock()
	defer n.Unlock()

	state, err := n.acceptorStore.Get(key)
	if err != nil && err.Error() == stableStoreNotFoundErr {
		// unfortunate way of handling errors
		// TODO: do better
		state, err = nil, nil
	}
	if err != nil {
		return AcceptorState{}, errors.Wrap(err, fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, n.ID))
	}

	acceptedBallotBytes, err := n.acceptorStore.Get(acceptedBallotKey(key))
	if err != nil && err.Error() == stableStoreNotFoundErr {
		// unfortunate way of handling errors
		// TODO: do better
		acceptedBallotBytes, err = nil, nil
	}
	if err != nil {
		return AcceptorState{State: state}, errors.Wrap(err, fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", n.ID))
	}

	var acceptedBallot Ballot
	if !bytes.Equal(acceptedBallotBytes, nil) {
		// ie we found an accepted Ballot
		acceptedBallotReader := bytes.NewReader(acceptedBallotBytes)
		dec := gob.NewDecoder(acceptedBallotReader)
		err = dec.Decode(&acceptedBallot)
		if err != nil {
			return AcceptorState{State: state}, errors.Wrap(err, fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", n.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if acceptedBallot.Counter > b.Counter {
			return AcceptorState{AcceptedBallot: acceptedBallot, State: state}, fmt.Errorf("submitted Ballot:%v is less than Ballot:%v of acceptor:%v", b, acceptedBallot, n.ID)
		}
	}

	promisedBallotBytes, err := n.acceptorStore.Get(promisedBallotKey(key))
	if err != nil && err.Error() == stableStoreNotFoundErr {
		// unfortunate way of handling errors
		// TODO: do better
		promisedBallotBytes, err = nil, nil
	}
	if err != nil {
		return AcceptorState{State: state, AcceptedBallot: acceptedBallot}, errors.Wrap(err, fmt.Sprintf("unable to get promisedBallot of acceptor:%v", n.ID))
	}
	var promisedBallot Ballot
	if !bytes.Equal(promisedBallotBytes, nil) {
		// ie we found an promised Ballot
		promisedBallotReader := bytes.NewReader(promisedBallotBytes)
		dec := gob.NewDecoder(promisedBallotReader)
		err = dec.Decode(&promisedBallot)
		if err != nil {
			return AcceptorState{State: state, AcceptedBallot: acceptedBallot}, errors.Wrap(err, fmt.Sprintf("unable to get promisedBallot of acceptor:%v", n.ID))
		}
		// TODO: also take into account the Node ID to resolve tie-breaks
		if promisedBallot.Counter > b.Counter {
			return AcceptorState{PromisedBallot: promisedBallot, AcceptedBallot: acceptedBallot, State: state}, fmt.Errorf("submitted Ballot:%v is less than Ballot:%v of acceptor:%v", b, promisedBallot, n.ID)
		}
	}

	// erase promised Ballot
	err = n.acceptorStore.Set(promisedBallotKey(key), nil)
	if err != nil {
		return AcceptorState{AcceptedBallot: acceptedBallot, State: state, PromisedBallot: promisedBallot}, errors.Wrap(err, fmt.Sprintf("unable to erase Ballot:%v", b))
	}

	var BallotBuffer bytes.Buffer
	enc := gob.NewEncoder(&BallotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return AcceptorState{AcceptedBallot: acceptedBallot, State: state}, errors.Wrap(err, fmt.Sprintf("unable to encode Ballot:%v", b))
	}
	// TODO. NB: it is possible, from the following logic, for an acceptor to accept a Ballot
	// but not accept the new state/value. ie if the call to acceptorStore.Set(acceptedBallotKey, BallotBuffer.Bytes()) succeeds
	// but acceptorStore.Set(key, state) fails.
	// we should think about the ramifications of that for a second.
	err = n.acceptorStore.Set(acceptedBallotKey(key), BallotBuffer.Bytes())
	if err != nil {
		return AcceptorState{AcceptedBallot: acceptedBallot, State: state}, errors.Wrap(err, fmt.Sprintf("unable to flush Ballot:%v to disk", b))
	}

	err = n.acceptorStore.Set(key, newState)
	if err != nil {
		return AcceptorState{AcceptedBallot: b, State: state}, errors.Wrap(err, fmt.Sprintf("unable to flush the new state:%v to disk", state))
	}

	return AcceptorState{AcceptedBallot: b, State: state}, nil

}
