package transport

import "github.com/komuw/kshaka/protocol"

// InmemTransport Implements the Transport interface, to allow kshaka/CASPaxos to be
// tested in-memory without going over a network.
type InmemTransport struct {
	NodeAddrress string
	NodePort     string
	Node         *protocol.Node
}

// TransportPrepare implements the Transport interface.
func (it *InmemTransport) TransportPrepare(b protocol.Ballot, key []byte) (protocol.AcceptorState, error) {
	return it.Node.Prepare(b, key)
}

// TransportAccept implements the Transport interface.
func (it *InmemTransport) TransportAccept(b protocol.Ballot, key []byte, state []byte) (protocol.AcceptorState, error) {
	return it.Node.Accept(b, key, state)
}
