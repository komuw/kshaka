package kshaka

import (
	"fmt"
)

const minimumNoAcceptors = 3

type prepareError string

func (e prepareError) Error() string {
	return string(e)
}

type client struct {
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
	id        uint64
	state     []byte
	ballot    ballot
	acceptors []*acceptor
}

func newProposer() proposer {
	var proposerID uint64 = 1
	b := ballot{counter: 1, proposerID: proposerID}
	p := proposer{id: proposerID, ballot: b}
	return p
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
	for _, v := range acceptedStates {
		fmt.Printf("\n\n v:%#+v\n", v)
	}
	return nil
}

type acceptorState struct {
	acceptedBallot ballot
	acceptedValue  []byte
}

// Acceptors store the accepted value; the system should have 2F+1 acceptors to tolerate F failures.
type acceptor struct {
	id            uint64
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
	a.acceptedState.acceptedBallot = b
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
