/*
Package kshaka is a pure Go implementation of the CASPaxos consensus protocol.
It's name is derived from the Kenyan hip hop group, Kalamashaka.

"CASPaxos is a replicated state machine (RSM) protocol. Unlike Raft and Multi-Paxos,
it doesn't use leader election and log replication, thus avoiding associated complexity.
Its symmetric peer-to-peer approach achieves optimal commit latency in wide-area networks
and doesn't cause transient unavailability when any [N−1] of N nodes crash." - The CASPaxos whitepaper, https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf

Example usage:

	// create a store that will be used. Ideally it should be a disk persisted store.
	// any store that implements hashicorp/raft StableStore interface will suffice
	kv := map[string][]byte{"foo": []byte("bar")}
	sStore := &InmemStore{kv: kv}

	// The function that will be applied by CASPaxos. This will be applied to the
	// current value stored under the key passed into the Propose method of the proposer.
	var setFunc = func(val []byte) ChangeFunction {
		return func(current []byte) ([]byte, error) {
			return val, nil
		}
	}

	// create a proposer with a list of acceptors
	p := &proposer{id: 1,
		ballot:     ballot{Counter: 1, ProposerID: 1},
		acceptors: []*acceptor{&acceptor{id: 1, stateStore: sStore},
			&acceptor{id: 2, stateStore: sStore},
			&acceptor{id: 3, stateStore: sStore},
			&acceptor{id: 4, stateStore: sStore}}}

	key := []byte("name")
	val := []byte("Masta-Ace")

	// make a proposition;
	// consensus via CASPaxos will happen and you will have your result back.
	newstate, err := p.Propose(key, setFunc(key, val))
	if err != nil {
		fmt.Printf("err: %v", err)
	}
	fmt.Printf("newstate: %v", newstate)


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

type acceptorState struct {
	promisedBallot ballot
	acceptedBallot ballot
	state          []byte
}

// Acceptors store the accepted value; the system should have 2F+1 acceptors to tolerate F failures.
// In general the "prepare" and "accept" operations affecting the same key should be mutually exclusive.
// How to achieve this is an implementation detail.
// eg in Gryadka it doesn't matter because the operations are implemented as Redis's stored procedures and Redis is single threaded. - Denis Rystsov

// Proposers perform the initialization by communicating with acceptors.
// Proposers keep minimal state needed to generate unique increasing update IDs (ballot numbers),
// the system may have arbitrary numbers of proposers.
type proposerAcceptor struct {
	id                uint64
	ballot            ballot
	proposerAcceptors []*proposerAcceptor

	// In general the "prepare" and "accept" operations affecting the same key should be mutually exclusive.
	// How to achieve this is an implementation detail.
	// eg in Gryadka it doesn't matter because the operations are implemented as Redis's stored procedures and Redis is single threaded. - Denis Rystsov
	sync.Mutex             // protects state
	stateStore StableStore //stores ballots and state
}

func newProposerAcceptor() *proposerAcceptor {
	p := &proposerAcceptor{}
	p.proposerAcceptors = []*proposerAcceptor{p}
	return p
}

func (p *proposerAcceptor) addproposerAcceptor(pa *proposerAcceptor) error {
	p.proposerAcceptors = append(p.proposerAcceptors, pa)
	return nil
}

// The proposer generates a ballot number, B, and sends ”prepare” messages containing that number(and it's ID) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// If all replies from acceptors contain the empty value, then the proposer defines the current state as ∅
// otherwise it picks the value of the tuple with the highest ballot number.
func (p *proposerAcceptor) sendPrepare(key []byte) ([]byte, error) {
	p.Lock()
	defer p.Unlock()

	var (
		noAcceptors         = len(p.proposerAcceptors)
		F                   = (noAcceptors - 1) / 2 // number of failures we can tolerate
		confirmationsNeeded = F + 1
		highBallotConfirm   ballot
		highBallotConflict  = p.ballot
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
	for _, a := range p.proposerAcceptors {
		go func(a *proposerAcceptor) {
			acceptedState, err := a.prepare(p.ballot, key)
			prepareResultChan <- prepareResult{acceptedState, err}
		}(a)
	}

	for i := 0; i < cap(prepareResultChan) && confirmationsNeeded > 0; i++ {
		res := <-prepareResultChan
		if res.err != nil {
			// conflict occured
			numberConflicts++
			if res.acceptedState.acceptedBallot.Counter > highBallotConflict.Counter {
				highBallotConflict = res.acceptedState.acceptedBallot
			} else if res.acceptedState.promisedBallot.Counter > highBallotConflict.Counter {
				highBallotConflict = res.acceptedState.promisedBallot
			}
		} else {
			// confirmation occured.
			numberConfirmations++
			if res.acceptedState.acceptedBallot.Counter >= highBallotConfirm.Counter {
				highBallotConfirm = res.acceptedState.acceptedBallot
				currentState = res.acceptedState.state
			}
			confirmationsNeeded--
		}
	}

	// we didn't get F+1 confirmations
	if numberConfirmations < confirmationsNeeded {
		p.ballot.Counter = highBallotConflict.Counter + 1
		return nil, prepareError(fmt.Sprintf("confirmations:%v is less than requires minimum of:%v", numberConfirmations, confirmationsNeeded))
	}

	return currentState, nil
}

// Proposer applies the f function to the current state and sends the result, new state,
// along with the generated ballot number B (an ”accept” message) to the acceptors.
// Proposer waits for the F + 1 confirmations.
// Proposer returns the new state to the client.
func (p *proposerAcceptor) sendAccept(key []byte, currentState []byte, changeFunc ChangeFunction) ([]byte, error) {

	/*
		Yes, acceptors should store tuple (promised ballot, accepted ballot and an accepted value) per key.
		Proposers, unlike acceptors, may use the same ballot number sequence.
		If we split a sequence of unique and increasing ballot numbers into several subsequences then any of them remains unique and increasing, so it's fine.
		- Rystsov
	*/

	// TODO: this locks are supposed to be per key
	// not method wide
	p.Lock()
	defer p.Unlock()
	var (
		noAcceptors         = len(p.proposerAcceptors)
		F                   = (noAcceptors - 1) / 2 // number of failures we can tolerate
		confirmationsNeeded = F + 1
		highBallotConflict  ballot
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
	for _, a := range p.proposerAcceptors {
		go func(a *proposerAcceptor) {
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
				highBallotConflict = res.acceptedState.acceptedBallot
			} else if res.acceptedState.promisedBallot.Counter > p.ballot.Counter {
				highBallotConflict = res.acceptedState.promisedBallot
			}
		} else {
			// confirmation occured.
			numberConfirmations++
			confirmationsNeeded--
		}
	}

	// we didn't get F+1 confirmations
	if numberConfirmations < confirmationsNeeded {
		p.ballot.Counter = highBallotConflict.Counter + 1
		return nil, acceptError(fmt.Sprintf("confirmations:%v is less than requires minimum of:%v", numberConfirmations, confirmationsNeeded))
	}

	return newState, nil
}

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

// Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
// Persists the ballot number as a promise and returns a confirmation either with an empty value (if it hasn’t accepted any value yet)
// or with a tuple of an accepted value and its ballot number.
func (a *proposerAcceptor) prepare(b ballot, key []byte) (acceptorState, error) {
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
func (a *proposerAcceptor) accept(b ballot, key []byte, state []byte) (acceptorState, error) {
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
	// but stateStore.Set(key, state) fails.
	// we should think about the ramifications of that for a second.
	err = a.stateStore.Set(acceptedBallotKey(key), ballotBuffer.Bytes())
	if err != nil {
		return acceptorState{acceptedBallot: acceptedBallot, state: state}, acceptError(fmt.Sprintf("%v", err))
	}
	fmt.Printf("\n\n key: %#+v. state: %#+v", string(key), string(state))
	err = a.stateStore.Set(key, state)
	if err != nil {
		return acceptorState{acceptedBallot: b, state: state}, acceptError(fmt.Sprintf("%v", err))
	}

	return acceptorState{acceptedBallot: b, state: state}, nil

}
