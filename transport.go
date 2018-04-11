package kshaka

import (
	"github.com/pkg/errors"
)

type Args struct {
	A, B int
}

type Quotient struct {
	Quo, Rem int
}

type Arith int

func (t *Arith) Multiply(args *Args, reply *int) error {
	*reply = args.A * args.B
	return nil
}

func (t *Arith) Divide(args *Args, quo *Quotient) error {
	if args.B == 0 {
		return errors.New("divide by zero")
	}
	quo.Quo = args.A / args.B
	quo.Rem = args.A % args.B
	return nil
}


arith := new(Arith)
rpc.Register(arith)
rpc.HandleHTTP()
l, e := net.Listen("tcp", ":1234")
if e != nil {
	log.Fatal("listen error:", e)
}
go http.Serve(l, nil)

type Transport interface{
	// Propose is the method that clients call when they want to submit
	// the f change function to a proposer.
	// It takes the key whose value you want to apply the ChangeFunction to
	// and also the ChangeFunction that will be applied to the value(contents) of that key.
	transportPropose(key []byte, changeFunc ChangeFunction) ([]byte, error)

	transportPrepare(b ballot, key []byte) (acceptorState, error)
	transportAccept(b ballot, key []byte, state []byte) (acceptorState, error)
}

// InmemTransport Implements the Transport interface, to allow kshaka/CASPaxos to be
// tested in-memory without going over a network.
type InmemTransport struct {
	nodeAddrress
	nodePort
}

/*
NetworkTransport provides a network based transport that can be
used to communicate with kshaka/CASPaxos on remote machines. It requires
an underlying stream layer to provide a stream abstraction, which can
be simple TCP, TLS, etc.
*/
type NetworkTransport struct {	
	nodeAddrress
	nodePort
}