package kshaka

import "fmt"

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
		fmt.Printf("error: %+v\n", err)
		return nil, err
	}

	// accept phase
	newState, err := prop.sendAccept(key, currentState, changeFunc)
	if err != nil {
		fmt.Printf("error: %+v\n", err)
		return nil, err
	}
	return newState, nil
}
