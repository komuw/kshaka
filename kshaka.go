package kshaka

import (
	"fmt"
)

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
	ballot    ballot
	acceptors []acceptor
}

func newProposer() proposer {
	var proposerID uint64 = 1
	b := ballot{counter: 1, proposerID: proposerID}
	p := proposer{id: proposerID, ballot: b}
	return p
}

func (p *proposer) addAcceptor(a acceptor) error {
	p.acceptors = append(p.acceptors, a)
	return nil
}

// Acceptors store the accepted value; the system should have 2F+1 acceptors to tolerate F failures.
type acceptor struct {
	id             uint64
	acceptedBallot ballot
	acceptedValue  []byte
}

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Persists the ballot number as a promise and returns a confirmation either with an empty value (if it hasn’t accepted any value yet)
// or with a tuple of an accepted value and its ballot number.
func (a *acceptor) prepare(b ballot) (ballot, []byte, error) {
	acceptedBallot := a.acceptedBallot
	acceptedValue := a.acceptedValue

	// TODO: also take into account the node ID
	// to resolve tie-breaks
	if acceptedBallot.counter > b.counter {
		return acceptedBallot, acceptedValue, fmt.Errorf("The submitted ballot: %v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.id)
	}
	a.acceptedBallot = b
	return acceptedBallot, acceptedValue, nil
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
