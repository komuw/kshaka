package protocol

// ProposerAcceptor is an entity that is both a proposer and an acceptor.
type ProposerAcceptor interface {
	proposer
	acceptor
	Propose(key []byte, changeFunc ChangeFunction) ([]byte, error)
	AddTransport(t Transport)
	AddMetadata(metadata map[string]string)
}
