package protocol

// Proposer perform the initialization by communicating with acceptors.
// Proposers keep minimal state needed to generate unique increasing update IDs (Ballot numbers),
// the system may have arbitrary numbers of proposers.
type proposer interface {
	sendPrepare(key []byte) ([]byte, error)
	sendAccept(key []byte, currentState []byte, changeFunc ChangeFunction) ([]byte, error)
}
