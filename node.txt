package kshaka

// Node represents an entity that is both a Proposer and an Acceptor.
// This is the thing that users who depend on this library will be creating and interacting with.
// A node is typically a server but it can also represent anything else you want.
// TODO
// type Node struct {
// 	id uint64
// 	proposer
// 	acceptor
// }

// func (n *Node) ala() {
// 	fmt.Println("NODE:", n)
// }

type Prop interface {
	sendPrepare(key []byte) ([]byte, error)
	sendAccept(key []byte, currentState []byte, changeFunc ChangeFunction) ([]byte, error)
}
type Accept interface {
	prepare(b ballot, key []byte) (acceptorState, error)
	accept(b ballot, key []byte, state []byte) (acceptorState, error)
}

type PropAccpt interface {
	Prop
	Accept
}

// Propose is the method that clients call when they want to submit
// the f change function to a proposer.
// It takes the key whose value you want to apply the ChangeFunction to and
// also the ChangeFunction that will be applied to the value(contents) of the key.
func Propose(prop PropAccpt, key []byte, changeFunc ChangeFunction) ([]byte, error) {
	// prepare phase
	currentState, err := prop.sendPrepare(key)
	if err != nil {
		return nil, err
	}

	// accept phase
	newState, err := prop.sendAccept(key, currentState, changeFunc)
	if err != nil {
		return nil, err
	}
	return newState, nil
}
