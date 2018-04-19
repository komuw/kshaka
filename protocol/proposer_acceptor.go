package protocol

// ProposerAcceptor is an entity that is both a proposer and an acceptor.
type ProposerAcceptor interface {
	proposer
	acceptor
	Propose(key []byte, changeFunc ChangeFunction) ([]byte, error)
}
