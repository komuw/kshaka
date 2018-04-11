package kshaka

// ProposerAcceptor is an entity that is both a proposer and an acceptor.
type ProposerAcceptor interface {
	proposer
	acceptor
	// Propose is the method that clients call when they want to submit
	// the f change function to a proposer.
	// It takes the key whose value you want to apply the ChangeFunction to
	// and also the ChangeFunction that will be applied to the value(contents) of that key.
	Propose(key []byte, changeFunc ChangeFunction) ([]byte, error)
}
