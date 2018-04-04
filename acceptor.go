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
func acceptedBallotKey(key []byte) []byte {
	return []byte(fmt.Sprintf("__ACCEPTED__BALLOT__KEY__207d1a68-34f3-11e8-88e5-cb7b2fa68526__3a39a980-34f3-11e8-853c-f35df5f3154e.%s", key))
}

func promisedBallotKey(key []byte) []byte {
	return []byte(fmt.Sprintf("__PROMISED__BALLOT__KEY__c8c07b0c-3598-11e8-98b8-97a4ad1feb35__d1a0ca9c-3598-11e8-9c5f-c3c66e6b4439.%s", key))
}

type acceptorState struct {
	promisedBallot ballot
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
func (a *acceptor) prepare(b ballot, key []byte) (acceptorState, error) {
	a.Lock()
	defer a.Unlock()

	state, err := a.stateStore.Get(key)
	if err != nil {
		return acceptorState{}, prepareError(fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, a.id))
	}

	acceptedBallotBytes, err := a.stateStore.Get(acceptedBallotKey(key))
	if err != nil {
		// TODO: propagate the underlying error
		return acceptorState{state: state}, prepareError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.id))
	}
	var acceptedBallot ballot
	if !bytes.Equal(acceptedBallotBytes, nil) {
		// ie we found an accepted ballot
		acceptedBallotReader := bytes.NewReader(acceptedBallotBytes)
		dec := gob.NewDecoder(acceptedBallotReader)
		err = dec.Decode(&acceptedBallot)
		if err != nil {
			return acceptorState{state: state}, prepareError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.id))
		}
		// TODO: also take into account the node ID to resolve tie-breaks
		if acceptedBallot.Counter > b.Counter {
			return acceptorState{acceptedBallot: acceptedBallot, state: state}, prepareError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.id))
		}
	}

	promisedBallotBytes, err := a.stateStore.Get(promisedBallotKey(key))
	if err != nil {
		// TODO: propagate the underlying error
		return acceptorState{state: state, acceptedBallot: acceptedBallot}, prepareError(fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.id))
	}
	var promisedBallot ballot
	if !bytes.Equal(promisedBallotBytes, nil) {
		// ie we found an promised ballot
		promisedBallotReader := bytes.NewReader(promisedBallotBytes)
		dec := gob.NewDecoder(promisedBallotReader)
		err = dec.Decode(&promisedBallot)
		if err != nil {
			return acceptorState{state: state, acceptedBallot: acceptedBallot}, prepareError(fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.id))
		}
		// TODO: also take into account the node ID to resolve tie-breaks
		if promisedBallot.Counter > b.Counter {
			return acceptorState{promisedBallot: promisedBallot, acceptedBallot: acceptedBallot, state: state}, prepareError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, promisedBallot, a.id))
		}
	}

	// TODO: this should be flushed to disk
	var ballotBuffer bytes.Buffer
	enc := gob.NewEncoder(&ballotBuffer)
	err = enc.Encode(b)
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: promisedBallot}, prepareError(fmt.Sprintf("%v", err))
	}

	err = a.stateStore.Set(promisedBallotKey(key), ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: promisedBallot}, prepareError(fmt.Sprintf("%v", err))
	}
	return acceptorState{acceptedBallot: acceptedBallot, state: state, promisedBallot: b}, nil
}

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Erases the promise, marks the received tuple (ballot number, value) as the accepted value and returns a confirmation
func (a *acceptor) accept(b ballot, key []byte, value []byte) (acceptorState, error) {
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

	state, err := a.stateStore.Get(key)
	if err != nil {
		return acceptorState{}, acceptError(fmt.Sprintf("unable to get state for key:%v from acceptor:%v", key, a.id))
	}

	acceptedBallotBytes, err := a.stateStore.Get(acceptedBallotKey(key))
	if err != nil {
		return acceptorState{state: state}, acceptError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.id))
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
			return acceptorState{state: state}, acceptError(fmt.Sprintf("unable to get acceptedBallot of acceptor:%v", a.id))
		}
		// TODO: also take into account the node ID to resolve tie-breaks
		if acceptedBallot.Counter > b.Counter {
			return acceptorState{acceptedBallot: acceptedBallot, state: state}, acceptError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, acceptedBallot, a.id))
		}
	}

	promisedBallotBytes, err := a.stateStore.Get(promisedBallotKey(key))
	if err != nil {
		// TODO: propagate the underlying error
		return acceptorState{state: state, acceptedBallot: acceptedBallot}, acceptError(fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.id))
	}
	var promisedBallot ballot
	if !bytes.Equal(promisedBallotBytes, nil) {
		// ie we found an promised ballot
		promisedBallotReader := bytes.NewReader(promisedBallotBytes)
		dec := gob.NewDecoder(promisedBallotReader)
		err = dec.Decode(&promisedBallot)
		if err != nil {
			return acceptorState{state: state, acceptedBallot: acceptedBallot}, acceptError(fmt.Sprintf("unable to get promisedBallot of acceptor:%v", a.id))
		}
		// TODO: also take into account the node ID to resolve tie-breaks
		if promisedBallot.Counter > b.Counter {
			return acceptorState{promisedBallot: promisedBallot, acceptedBallot: acceptedBallot, state: state}, acceptError(fmt.Sprintf("submitted ballot:%v is less than ballot:%v of acceptor:%v", b, promisedBallot, a.id))
		}
	}

	// erase promised ballot
	err = a.stateStore.Set(promisedBallotKey(key), nil)
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
	// but not accept the new state/value. ie if the call to stateStore.Set(acceptedBallotKey, ballotBuffer.Bytes()) succeeds
	// but stateStore.Set(key, value) fails.
	// we should think about the ramifications of that for a second.
	err = a.stateStore.Set(acceptedBallotKey(key), ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, acceptError(fmt.Sprintf("%v", err))
	}
	fmt.Printf("\n\n key: %#+v. value: %#+v", string(key), string(value))
	err = a.stateStore.Set(key, value)
	if err != nil {
		return acceptorState{acceptedBallot: b, state: state}, acceptError(fmt.Sprintf("%v", err))
	}

	return acceptorState{acceptedBallot: b, state: value}, nil

}
