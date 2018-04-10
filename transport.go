package kshaka

import "net"

type RPCrequest struct {
	key          []byte
	currentState []byte
	ballot       ballot
	changeFunc   ChangeFunction
}

type RPCresponse struct {
	currentState  []byte
	newState      []byte
	acceptedState acceptorState
}

// Transport provides an interface for network transports
// to allow kshaka/CASPaxos to communicate with other nodes.
type Transport interface {
	// SendRPC sends the appropriate RPC to the target node.
	SendRPC(nodeID uint64, nodeAddress net.IP, rpcMethod string, req RPCrequest, resp *RPCresponse) error
}
