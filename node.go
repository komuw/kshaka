package kshaka

// Node represents an entity that is both a Proposer and an Acceptor.
// This is the thing that users who depend on this library will be creating and interacting with.
// A node is typically a server but it can also represent anything else you want.
// TODO
type Node struct {
	id uint64
	proposer
	acceptor
}
