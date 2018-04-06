package kshaka

type proposer interface {
	sendPrepare(key []byte) ([]byte, error)
	sendAccept(key []byte, currentState []byte, changeFunc ChangeFunction) ([]byte, error)
}
