/*
Package kshaka is a pure Go implementation of the CASPaxos consensus protocol.
It's name is derived from the Kenyan hip hop group, Kalamashaka.

"CASPaxos is a replicated state machine (RSM) protocol. Unlike Raft and Multi-Paxos,
it doesn’t use leader election and log replication, thus avoiding associated complexity.
Its symmetric peer-to-peer approach achieves optimal commit latency in wide-area networks
and doesn’t cause transient unavailability when any [N−1] of N nodes crash." - [The CASPaxos whitepaper](https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf)

TODO: add system design here.
*/
package kshaka

// It’s convenient to use tuples as ballot numbers.
// To generate it a proposer combines its numerical ID with a local increasing counter: (counter, ID).
// To compare ballot tuples, we should compare the first component of the tuples and use ID only as a tiebreaker.
// TODO: make ballot a simple structure, like uint64, so that we dont have to use encoding/gob when saving it.
type ballot struct {
	Counter    uint64
	ProposerID uint64
}

// monotonically increase the ballot
func (p *proposer) incBallot() {
	p.ballot.Counter++
}
