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
		acceptors: []*acceptor{&acceptor{id: 1, acceptorStore: sStore},
			&acceptor{id: 2, acceptorStore: sStore},
			&acceptor{id: 3, acceptorStore: sStore},
			&acceptor{id: 4, acceptorStore: sStore}}}

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

// ProposerAcceptor is an entity that is both a proposer and an acceptor.
type ProposerAcceptor interface {
	proposer
	acceptor
}

// Propose is the method that clients call when they want to submit
// the f change function to a proposer.
// It takes the key whose value you want to apply the ChangeFunction to and
// also the ChangeFunction that will be applied to the value(contents) of the key.
func Propose(prop ProposerAcceptor, key []byte, changeFunc ChangeFunction) ([]byte, error) {
	// prepare phase
	currentState, err := prop.sendPrepare(key)
	if err != nil {
		return nil, err
	}

	// accept phase
	newState, err := prop.sendAccept(key, currentState, changeFunc)
	if err != nil {
		return nil, err
	}
	return newState, nil
}
