package kshaka

import (
	"net/rpc"
)

// type Args struct {
// 	A, B int
// }

// type Quotient struct {
// 	Quo, Rem int
// }

// type Arith int

// func (t *Arith) Multiply(args *Args, reply *int) error {
// 	*reply = args.A * args.B
// 	return nil
// }

// func (t *Arith) Divide(args *Args, quo *Quotient) error {
// 	if args.B == 0 {
// 		return errors.New("divide by zero")
// 	}
// 	quo.Quo = args.A / args.B
// 	quo.Rem = args.A % args.B
// 	return nil
// }

// arith := new(Arith)
// rpc.Register(arith)
// rpc.HandleHTTP()
// l, e := net.Listen("tcp", ":1234")
// if e != nil {
// 	log.Fatal("listen error:", e)
// }
// go http.Serve(l, nil)

// client, err := rpc.DialHTTP("tcp", serverAddress + ":1234")
// if err != nil {
// 	log.Fatal("dialing:", err)
// }
// // Then it can make a remote call:
// // Synchronous call
// args := &server.Args{7,8}
// var reply int
// err = client.Call("Arith.Multiply", args, &reply)
// if err != nil {
// 	log.Fatal("arith error:", err)
// }
// fmt.Printf("Arith: %d*%d=%d", args.A, args.B, reply)

type Transport interface {
	// Propose is the method that clients call when they want to submit
	// the f change function to a proposer.
	// It takes the key whose value you want to apply the ChangeFunction to
	// and also the ChangeFunction that will be applied to the value(contents) of that key.
	TransportPropose(key []byte, changeFunc ChangeFunction) ([]byte, error)

	TransportPrepare(b ballot, key []byte) (acceptorState, error)
	TransportAccept(b ballot, key []byte, state []byte) (acceptorState, error)
}

// InmemTransport Implements the Transport interface, to allow kshaka/CASPaxos to be
// tested in-memory without going over a network.
type InmemTransport struct {
	NodeAddrress string
	NodePort     string
	Node         *Node
}

func (it *InmemTransport) TransportPrepare(b ballot, key []byte) (acceptorState, error) {
	return it.Node.prepare(b, key)

}
func (it *InmemTransport) TransportAccept(b ballot, key []byte, state []byte) (acceptorState, error) {
	return it.Node.accept(b, key, state)
}
func (it *InmemTransport) TransportPropose(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	return it.Node.propose(key, changeFunc)
}

/*
NetworkTransport provides a network based transport that can be
used to communicate with kshaka/CASPaxos on remote machines. It requires
an underlying stream layer to provide a stream abstraction, which can
be simple TCP, TLS, etc.
*/
type NetworkTransport struct {
	NodeAddrress string
	NodePort     string
}

func (nt *NetworkTransport) TransportPrepare(b ballot, key []byte) (acceptorState, error) {
	_, err := rpc.DialHTTP("tcp", nt.NodeAddrress+nt.NodePort)
	if err != nil {
		return acceptorState{}, err
	}

	// makes a Synchronous call. there are also async versions of this??
	// err = client.Call("Arith.Multiply", args, &reply)
	// if err != nil {
	// 	return acceptorState{}, err
	// }
	return acceptorState{}, nil
}

func (nt *NetworkTransport) TransportAccept(b ballot, key []byte, state []byte) (acceptorState, error) {
	_, err := rpc.DialHTTP("tcp", nt.NodeAddrress+nt.NodePort)
	if err != nil {
		return acceptorState{}, err
	}

	// makes a Synchronous call. there are also async versions of this??
	// err = client.Call("Arith.Multiply", args, &reply)
	// if err != nil {
	// 	return acceptorState{}, err
	// }
	return acceptorState{}, nil

}
