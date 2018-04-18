package kshaka

// It’s convenient to use tuples as Ballot numbers.
// To generate it a proposer combines its numerical ID with a local increasing counter: (counter, ID).
// To compare Ballot tuples, we should compare the first component of the tuples and use ID only as a tiebreaker.
// TODO: make Ballot a simple structure, like uint64, so that we dont have to use encoding/gob when saving it.
type Ballot struct {
	Counter            uint64
	ProposerAcceptorID uint64
}
