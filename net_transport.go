package kshaka

import (
	"fmt"
	"net/rpc"
	"sync"

	"github.com/pkg/errors"
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
	nodeAddress string
	nodeport    string
}

// NewNetworkTransport is used to initialize a new transport
func NewNetworkTransport(nodeID uint64, nodeAddress string) *NetworkTransport {
	return &NetworkTransport{nodeID: nodeID, nodeAddress: nodeAddress, nodeport: "15000"}
}

// SendRPC sends the appropriate RPC to the target node.
func (n *NetworkTransport) SendRPC(rpcMethod string, req RPCrequest, resp *RPCresponse) error {
	// TODO: add security??
	client, err := rpc.DialHTTP("tcp", n.nodeAddress+n.nodeport)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to dial node ID:%v, address:%v", n.nodeID, n.nodeAddress))
	}
	err = client.Call(rpcMethod, req, resp)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("unable to call rpcMethod:%v of node ID:%v, address:%v", rpcMethod, n.nodeID, n.nodeAddress))
	}
	return nil
}
