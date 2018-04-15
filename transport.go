package kshaka

import (
	"fmt"
	"net/http"
	"time"
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

	TransportPrepare(b ballot, key []byte) (AcceptorState, error)
	TransportAccept(b ballot, key []byte, state []byte) (AcceptorState, error)
}

// InmemTransport Implements the Transport interface, to allow kshaka/CASPaxos to be
// tested in-memory without going over a network.
type InmemTransport struct {
	NodeAddrress string
	NodePort     string
	Node         *Node
}

func (it *InmemTransport) TransportPrepare(b ballot, key []byte) (AcceptorState, error) {
	return it.Node.prepare(b, key)

}
func (it *InmemTransport) TransportAccept(b ballot, key []byte, state []byte) (AcceptorState, error) {
	return it.Node.accept(b, key, state)
}
func (it *InmemTransport) TransportPropose(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	return it.Node.propose(key, changeFunc)
}

/*
HttpTransport provides a network based transport that can be
used to communicate with kshaka/CASPaxos on remote machines. It requires
an underlying stream layer to provide a stream abstraction, which can
be simple TCP, TLS, etc.
*/
type HttpTransport struct {
	NodeAddrress string
	NodePort     string
	URI          string
}

var HttpTransportClientTimeout = time.Second * 3
var HttpTransportClient = &http.Client{Timeout: HttpTransportClientTimeout}

func (ht *HttpTransport) TransportPrepare(b ballot, key []byte) (AcceptorState, error) {
	//   response, _ := netClient.Get(url)

	return AcceptorState{}, nil
}

func (ht *HttpTransport) TransportAccept(b ballot, key []byte, state []byte) (AcceptorState, error) {
	return AcceptorState{}, nil
}

func (ht *HttpTransport) TransportPropose(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	return nil, nil
}

func (n *Node) ProposeRPC(args *ProposeArgs, newState *[]byte) error {
	fmt.Println("Proposition called::")
	fmt.Println("Key, ChangeFunc::", args)
	fmt.Println("newState::", newState)
	fmt.Println()
	s, err := n.propose(args.Key, args.ChangeFunc)
	if err != nil {
		fmt.Println("Proposition error::", err)
		return err
	}
	newState = &s
	return nil
}
