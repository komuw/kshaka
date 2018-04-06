package kshaka

import (
	"fmt"
)

// TODO: handle zero values of stuff. eg if we find that an acceptor has replied with a state of default []byte(ie <nil>)
// then we probably shouldn't save that as the state or reply to client as the state.
// or maybe we should??
// mull on this.
const minimumNoAcceptors = 3

// acceptedBallotKey is the key that we use to store the value of the current accepted ballot.
// it ought to be unique and clients/users will be prohibited from using this value as a key for their data.
func acceptedBallotKey(key []byte) []byte {
	return []byte(fmt.Sprintf("__ACCEPTED__BALLOT__KEY__207d1a68-34f3-11e8-88e5-cb7b2fa68526__3a39a980-34f3-11e8-853c-f35df5f3154e.%s", key))
}

func promisedBallotKey(key []byte) []byte {
	return []byte(fmt.Sprintf("__PROMISED__BALLOT__KEY__c8c07b0c-3598-11e8-98b8-97a4ad1feb35__d1a0ca9c-3598-11e8-9c5f-c3c66e6b4439.%s", key))
}

type acceptorState struct {
	promisedBallot ballot
	acceptedBallot ballot
	state          []byte
}
type acceptor interface {
	prepare(b ballot, key []byte) (acceptorState, error)
	accept(b ballot, key []byte, state []byte) (acceptorState, error)
}
