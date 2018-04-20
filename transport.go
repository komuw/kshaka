package kshaka

// Transport provides an interface for network transports
// to allow kshaka/CASPaxos to communicate with other nodes.
// An example is github.com/komuw/kshaka/httpTransport
type Transport interface {
	TransportPrepare(b Ballot, key []byte) (AcceptorState, error)
	TransportAccept(b Ballot, key []byte, state []byte) (AcceptorState, error)
}
