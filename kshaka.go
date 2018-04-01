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
	"bytes"
	"encoding/gob"
	"fmt"
	"sync"
)

// TODO: handle zero values of stuff. eg if we find that an acceptor has replied with a state of default []byte(ie <nil>)
// then we probably shouldn't save that as the state or reply to client as the state.
// or maybe we should??
// mull on this.

const minimumNoAcceptors = 3

// acceptedBallotKey is the key that we use to store the value of the current accepted ballot.
// it ought to be unique and clients/users will be prohibited from using this value as a key for their data.
var acceptedBallotKey = []byte("__ACCEPTED__BALLOT__KEY__207d1a68-34f3-11e8-88e5-cb7b2fa68526__3a39a980-34f3-11e8-853c-f35df5f3154e__")

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

// It’s convenient to use tuples as ballot numbers.
// To generate it a proposer combines its numerical ID with a local increasing counter: (counter, ID).
// To compare ballot tuples, we should compare the first component of the tuples and use ID only as a tiebreaker.
// TODO: make ballot a simple structure, like uint64, so that we dont have to use encoding/gob when saving it.
type ballot struct {
	Counter    uint64
	ProposerID uint64
}

// Proposers perform the initialization by communicating with acceptors.
// Proposers keep minimal state needed to generate unique increasing update IDs (ballot numbers),
// the system may have arbitrary numbers of proposers.
type proposer struct {
	id        uint64
	ballot    ballot
	acceptors []*acceptor

	// In general the "prepare" and "accept" operations affecting the same key should be mutually exclusive.
	// How to achieve this is an implementation detail.
	// eg in Gryadka it doesn't matter because the operations are implemented as Redis's stored procedures and Redis is single threaded. - Denis Rystsov
	sync.Mutex // protects state
	stateStore StableStore
}

func newProposer() *proposer {
	var proposerID uint64 = 1
	b := ballot{Counter: 1, ProposerID: proposerID}
	p := proposer{id: proposerID, ballot: b}
	return &p
}

func (p *proposer) addAcceptor(a *acceptor) error {
	p.acceptors = append(p.acceptors, a)
	return nil
}

// Propose is the fucntion that clients call when they want to client submits
// the f change function to a proposer.
func (p *proposer) Propose(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	// prepare phase
	err := p.sendPrepare(key)
	if err != nil {
		return nil, err
	}

	// accept phase
	newState, err := p.sendAccept(key, changeFunc)
	if err != nil {
		return nil, err
	}
	return newState, nil

}

// The proposer generates a ballot number, B, and sends ”prepare” messages containing that number(and it's ID) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// If all replies from acceptors contain the empty value, then the proposer defines the current state as ∅
// otherwise it picks the value of the tuple with the highest ballot number.
func (p *proposer) sendPrepare(key []byte) error {
	p.Lock()
	defer p.Unlock()

	noAcceptors := len(p.acceptors)
	if noAcceptors < minimumNoAcceptors {
		return prepareError(fmt.Sprintf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors))
	}
	if bytes.Equal(key, acceptedBallotKey) {
		return prepareError(fmt.Sprintf("the key:%v is reserved for storing kshaka internal state. chose another key.", acceptedBallotKey))
	}

	// number of failures we can tolerate:
	F := (noAcceptors - 1) / 2

	acceptedStates := []acceptorState{}
	OKs := []bool{}
	var err error

	for _, a := range p.acceptors {
		fmt.Printf("acceptor %#+v\n", a)
		//TODO: call prepare concurrently
		acceptedState, prepareOK, e := a.prepare(p.ballot, key)
		err = e
		acceptedStates = append(acceptedStates, acceptedState)
		if prepareOK {
			// we only count confirmations as those that replied with true
			OKs = append(OKs, prepareOK)
		}
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
		if v.acceptedBallot.Counter > maxState.acceptedBallot.Counter {
			maxState = v
		}
	}

	err = p.stateStore.Set(key, maxState.state)
	if err != nil {
		return prepareError(fmt.Sprintf("%v", err))
	}
	fmt.Printf("\n\n maxState:%#+v\n", maxState)
	return nil
}

// Proposer applies the f function to the current state and sends the result, new state,
// along with the generated ballot number B (an ”accept” message) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// Proposer returns the new state to the client.
func (p *proposer) sendAccept(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	p.Lock()
	defer p.Unlock()

	// probably we shouldn't call this method, sendAccept, if we havent called prepare yet and it is finished
	noAcceptors := len(p.acceptors)
	if noAcceptors < minimumNoAcceptors {
		return nil, acceptError(fmt.Sprintf("number of acceptors:%v is less than required minimum of:%v", noAcceptors, minimumNoAcceptors))
	}
	if bytes.Equal(key, acceptedBallotKey) {
		return nil, acceptError(fmt.Sprintf("the key:%v is reserved for storing kshaka internal state. chose another key.", acceptedBallotKey))
	}

	// number of failures we can tolerate:
	F := (noAcceptors - 1) / 2

	acceptedStates := []acceptorState{}
	OKs := []bool{}
	var err error

	value, err := changeFunc(key, p.stateStore)
	if err != nil {
		return nil, acceptError(fmt.Sprintf("%v", err))
	}
	// TODO: if value == nil should we save it, or return error??
	// think about this some more

	for _, a := range p.acceptors {
		fmt.Printf("acceptor %#+v\n", a)
		//TODO: call prepare concurrently
		acceptedState, acceptOK, e := a.accept(p.ballot, key, value)
		err = e
		acceptedStates = append(acceptedStates, acceptedState)
		if acceptOK {
			// we only count confirmations as those that replied with true
			OKs = append(OKs, acceptOK)
		}
	}
	fmt.Println("acceptedStates, OKs, err, F", acceptedStates, OKs, err, F)

	// TODO: implement better logic for waiting for F+1 confirmations
	if len(OKs) < F+1 {
		return nil, acceptError(fmt.Sprintf("confirmations:%v is less than requires minimum of:%v", len(OKs), F+1))
	}

	err = p.stateStore.Set(key, value)
	if err != nil {
		return nil, acceptError(fmt.Sprintf("%v", err))
	}
	fmt.Printf("\n\n newState:%#+v\n", p.stateStore)
	return value, nil
}

type acceptorState struct {
	acceptedBallot ballot
	state          []byte
}

// Acceptors store the accepted value; the system should have 2F+1 acceptors to tolerate F failures.
type acceptor struct {
	id uint64

	// In general the "prepare" and "accept" operations affecting the same key should be mutually exclusive.
	// How to achieve this is an implementation detail.
	// eg in Gryadka it doesn't matter because the operations are implemented as Redis's stored procedures and Redis is single threaded. - Denis Rystsov
	sync.Mutex             // protects stateStore
	stateStore StableStore //stores ballots and state
}

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Persists the ballot number as a promise and returns a confirmation either with an empty value (if it hasn’t accepted any value yet)
// or with a tuple of an accepted value and its ballot number.
func (a *acceptor) prepare(b ballot, key []byte) (acceptorState, bool, error) {
	a.Lock()
	defer a.Unlock()

	state, err := a.stateStore.Get(key)
	if err != nil {
		return acceptorState{}, false, prepareError(fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, a.id))
	}

	acceptedBallotBytes, err := a.stateStore.Get(acceptedBallotKey)
	acceptedBallotReader := bytes.NewReader(acceptedBallotBytes)

	var acceptedBallot ballot
	dec := gob.NewDecoder(acceptedBallotReader)
	err = dec.Decode(&acceptedBallot)
	if err != nil {
		return acceptorState{state: state}, false, prepareError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.id))
	}
	// TODO: also take into account the node ID
	// to resolve tie-breaks
	if acceptedBallot.Counter > b.Counter {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, false, prepareError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.id))
	}

	// TODO: this should be flushed to disk
	var ballotBuffer bytes.Buffer
	enc := gob.NewEncoder(&ballotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, false, prepareError(fmt.Sprintf("%v", err))
	}

	err = a.stateStore.Set(acceptedBallotKey, ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, false, prepareError(fmt.Sprintf("%v", err))
	}
	return acceptorState{acceptedBallot: b, state: state}, true, nil
}

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Erases the promise, marks the received tuple (ballot number, value) as the accepted value and returns a confirmation
func (a *acceptor) accept(b ballot, key []byte, value []byte) (acceptorState, bool, error) {
	// we still need to unlock even when using a StableStore as the store of state.
	// this is because, someone may provide us with non-concurrent safe StableStore
	a.Lock()
	defer a.Unlock()

	state, err := a.stateStore.Get(key)
	if err != nil {
		return acceptorState{}, false, acceptError(fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, a.id))
	}

	acceptedBallotBytes, err := a.stateStore.Get(acceptedBallotKey)
	if err != nil {
		return acceptorState{state: state}, false, acceptError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.id))
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
			return acceptorState{state: state}, false, acceptError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.id))
		}
		if acceptedBallot.Counter > b.Counter {
			return acceptorState{acceptedBallot: acceptedBallot, state: state}, false, acceptError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.id))
		}
	}
	// TODO: this should be flushed to disk
	var ballotBuffer bytes.Buffer
	enc := gob.NewEncoder(&ballotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, false, acceptError(fmt.Sprintf("%v", err))
	}

	// TODO. NB: it is possible, from the following logic, for an acceptor to accept a ballot
	// but not accept the new state/value. ie if the call to stateStore.Set(acceptedBallotKey, ballotBuffer.Bytes()) succeeds
	// but stateStore.Set(key, value) fails.
	// we should think about the ramifications of that for a second.
	err = a.stateStore.Set(acceptedBallotKey, ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, false, acceptError(fmt.Sprintf("%v", err))
	}
	fmt.Printf("\n\n key: %#+v. value: %#+v", string(key), string(value))
	err = a.stateStore.Set(key, value)
	if err != nil {
		return acceptorState{acceptedBallot: b, state: state}, false, acceptError(fmt.Sprintf("%v", err))
	}

	return acceptorState{acceptedBallot: b, state: value}, true, nil

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
