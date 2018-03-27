/*
Package kshaka is a pure Go implementation of the CASPaxos consensus protocol.
It's name is derived from the Kenyan hip hop group, Kalamashaka.

"CASPaxos is a replicated state machine (RSM) protocol. Unlike Raft and Multi-Paxos,
it doesn’t use leader election and log replication, thus avoiding associated complexity.
Its symmetric peer-to-peer approach achieves optimal commit latency in wide-area networks
and doesn’t cause transient unavailability when any [N−1] of N nodes crash." - [The CASPaxos whitepaper](https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf)

TODO: add system design here.
*/
package kshaka

import (
	"fmt"
	"sync"
)

const minimumNoAcceptors = 3

type prepareError string

func (e prepareError) Error() string {
	return string(e)
}

type acceptError string

func (e acceptError) Error() string {
	return string(e)
}

type client struct {
}

// TODO: remove this placeholder for the; f change function
var changeFunc = func(state []byte) []byte {
	return state
}

// It’s convenient to use tuples as ballot numbers.
// To generate it a proposer combines its numerical ID with a local increasing counter: (counter, ID).
// To compare ballot tuples, we should compare the first component of the tuples and use ID only as a tiebreaker.
type ballot struct {
	counter    uint64
	proposerID uint64
}

// Proposers perform the initialization by communicating with acceptors.
// Proposers keep minimal state needed to generate unique increasing update IDs (ballot numbers),
// the system may have arbitrary numbers of proposers.
type proposer struct {
	id         uint64
	ballot     ballot
	acceptors  []*acceptor
	sync.Mutex // protects state
	state      []byte
}

func newProposer() *proposer {
	var proposerID uint64 = 1
	b := ballot{counter: 1, proposerID: proposerID}
	p := proposer{id: proposerID, ballot: b}
	return &p
}

func (p *proposer) addAcceptor(a *acceptor) error {
	p.acceptors = append(p.acceptors, a)
	return nil
}

// The proposer generates a ballot number, B, and sends ”prepare” messages containing that number(and it's ID) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// If all replies from acceptors contain the empty value, then the proposer defines the current state as ∅
// otherwise it picks the value of the tuple with the highest ballot number.
func (p *proposer) sendPrepare() error {
	noAcceptors := len(p.acceptors)
	if noAcceptors < minimumNoAcceptors {
		return prepareError(fmt.Sprintf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors))
	}
	// number of failures we can tolerate:
	F := (noAcceptors - 1) / 2

	acceptedStates := []acceptorState{}
	OKs := []bool{}
	var err error

	for _, a := range p.acceptors {
		fmt.Printf("acceptor %#+v\n", a)
		//TODO: call prepare concurrently
		acceptedState, prepareOK, e := a.prepare(p.ballot)
		acceptedStates = append(acceptedStates, acceptedState)
		OKs = append(OKs, prepareOK)
		err = e
	}
	fmt.Println("acceptedStates, OKs, err, F", acceptedStates, OKs, err, F)

	// TODO: implement better logic for waiting for F+1 confirmations
	if len(OKs) < F+1 {
		return prepareError(fmt.Sprintf("confirmations:%v is less than requires minimum of:%v", len(OKs), F+1))
	}

	// Note; even if all replies from acceptors contain the empty value,
	// then, p.state would be equal to the default value of []byte
	maxState := acceptorState{}
	for _, v := range acceptedStates {
		if v.acceptedBallot.counter > maxState.acceptedBallot.counter {
			maxState = v
		}
	}
	p.Lock()
	p.state = maxState.acceptedValue
	p.Unlock()
	fmt.Printf("\n\n maxState:%#+v\n", maxState)
	return nil
}

// Proposer applies the f function to the current state and sends the result, new state,
// along with the generated ballot number B (an ”accept” message) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// Proposer returns the new state to the client.
func (p *proposer) sendAccept() ([]byte, error) {
	// probably we shouldn't call this method if we havent called prepare yet and it is finished
	noAcceptors := len(p.acceptors)
	if noAcceptors < minimumNoAcceptors {
		return nil, acceptError(fmt.Sprintf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors))
	}
	// number of failures we can tolerate:
	F := (noAcceptors - 1) / 2

	acceptedStates := []acceptorState{}
	OKs := []bool{}
	var err error

	newState := changeFunc(p.state)
	for _, a := range p.acceptors {
		fmt.Printf("acceptor %#+v\n", a)
		//TODO: call prepare concurrently
		acceptedState, acceptOK, e := a.accept(p.ballot, newState)
		acceptedStates = append(acceptedStates, acceptedState)
		OKs = append(OKs, acceptOK)
		err = e
	}
	fmt.Println("acceptedStates, OKs, err, F", acceptedStates, OKs, err, F)

	// TODO: implement better logic for waiting for F+1 confirmations
	if len(OKs) < F+1 {
		return nil, acceptError(fmt.Sprintf("confirmations:%v is less than requires minimum of:%v", len(OKs), F+1))
	}

	p.Lock()
	p.state = newState
	p.Unlock()
	fmt.Printf("\n\n newState:%#+v\n", p.state)
	return p.state, nil
}

type acceptorState struct {
	acceptedBallot ballot
	acceptedValue  []byte
}

// Acceptors store the accepted value; the system should have 2F+1 acceptors to tolerate F failures.
type acceptor struct {
	id            uint64
	sync.Mutex    // protects acceptedState
	acceptedState acceptorState
}

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Persists the ballot number as a promise and returns a confirmation either with an empty value (if it hasn’t accepted any value yet)
// or with a tuple of an accepted value and its ballot number.
func (a *acceptor) prepare(b ballot) (acceptorState, bool, error) {
	// TODO: also take into account the node ID
	// to resolve tie-breaks
	if a.acceptedState.acceptedBallot.counter > b.counter {
		return a.acceptedState, false, prepareError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, a.acceptedState.acceptedBallot, a.id))
	}

	// TODO: this should be flushed to disk
	a.Lock()
	a.acceptedState.acceptedBallot = b
	a.Unlock()
	return a.acceptedState, true, nil
}

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Erases the promise, marks the received tuple (ballot number, value) as the accepted value and returns a confirmation
func (a *acceptor) accept(b ballot, newState []byte) (acceptorState, bool, error) {
	if a.acceptedState.acceptedBallot.counter > b.counter {
		return a.acceptedState, false, acceptError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, a.acceptedState.acceptedBallot, a.id))
	}

	// TODO: this should be flushed to disk
	a.Lock()
	a.acceptedState.acceptedBallot = b
	a.acceptedState.acceptedValue = newState
	a.Unlock()
	return a.acceptedState, true, nil

}

// Node represents an entity that is both a Proposer and an Acceptor.
// This is the thing that users who depend on this library will be creating and interacting with.
// A node is typically a server but it can also represent anything else you want.
// TODO
type Node struct {
	id uint64
	proposer
	acceptor
}
