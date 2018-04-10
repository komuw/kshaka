package kshaka

import (
	"net"
	"sync"
)

/*
NetworkTransport provides a network based transport that can be
used to communicate with Kshaka/CASPaxos on remote machines. It requires
an underlying stream layer to provide a stream abstraction, which can
be simple TCP, TLS, etc.
This transport is very simple and lightweight. Each RPC request is
framed by sending a byte that indicates the message type, followed
by the MsgPack encoded request.
The response is an error string followed by the response object,
both are encoded using MsgPack.
*/
// TODO: edit to reflect the correct state
// eg; remove reference to MsgPack
type NetworkTransport struct {
	sync.RWMutex
	nodeID      uint64
	nodeAddress net.IP
}

// NewNetworkTransport is used to initialize a new transport
func NewNetworkTransport(nodeID uint64, nodeAddress net.IP) *NetworkTransport {
	return &NetworkTransport{nodeID: nodeID, nodeAddress: nodeAddress}
}


SendRPC(nodeID uint64, nodeAddress net.IP, rpcMethod string, req RPCrequest, resp *RPCresponse) error

// SendRPC sends the appropriate RPC to the target node.
func (n *NetworkTransport) SendRPC(nodeID uint64, nodeAddress net.IP, rpcMethod string, req RPCrequest, resp *RPCresponse) error {
	// Get a conn
	conn, err := n.getConnFromAddressProvider(id, target)
	if err != nil {
		return err
	}

	// Set a deadline
	if n.timeout > 0 {
		conn.conn.SetDeadline(time.Now().Add(n.timeout))
	}

	// Send the RPC
	if err = sendRPC(conn, rpcType, args); err != nil {
		return err
	}

	// Decode the response
	canReturn, err := decodeResponse(conn, resp)
	if canReturn {
		n.returnConn(conn)
	}
	return err
}


// DOC:: https://golang.org/pkg/net/rpc
client, err := rpc.DialHTTP("tcp", serverAddress + ":1234")
if err != nil {
	log.Fatal("dialing:", err)
}

//Then it can make a remote call:
// Synchronous call
args := &server.Args{7,8}
var reply int
err = client.Call("Arith.Multiply", args, &reply)
if err != nil {
	log.Fatal("arith error:", err)
}
fmt.Printf("Arith: %d*%d=%d", args.A, args.B, reply)