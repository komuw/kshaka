package kshaka

type Transport interface {
	TransportPrepare(b Ballot, key []byte) (AcceptorState, error)
	TransportAccept(b Ballot, key []byte, state []byte) (AcceptorState, error)
}

// InmemTransport Implements the Transport interface, to allow kshaka/CASPaxos to be
// tested in-memory without going over a network.
type InmemTransport struct {
	NodeAddrress string
	NodePort     string
	Node         *Node
}

func (it *InmemTransport) TransportPrepare(b Ballot, key []byte) (AcceptorState, error) {
	return it.Node.Prepare(b, key)
}
func (it *InmemTransport) TransportAccept(b Ballot, key []byte, state []byte) (AcceptorState, error) {
	return it.Node.Accept(b, key, state)
}
