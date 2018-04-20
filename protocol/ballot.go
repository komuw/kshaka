package kshaka

// Ballot is unique increasing updateID
// It’s convenient to use tuples as Ballot numbers.
// To generate it a proposer combines its numerical ID with a local increasing counter: (counter, ID).
// To compare Ballot tuples, we should compare the first component of the tuples and use ID only as a tiebreaker.
type Ballot struct {
	Counter uint64
	NodeID  uint64
}
