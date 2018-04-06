package kshaka

import (
	"testing"
)

func TestNode_incBallot(t *testing.T) {
	kv := map[string][]byte{"": []byte("")}
	store := &InmemStore{kv: kv}

	tests := []struct {
		name string
		n    *Node
	}{
		{name: "increment ballot", n: newNode(store)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.n
			n.incBallot()
			n.incBallot()
			n.incBallot()

			if n.ballot.Counter != 3 {
				t.Errorf("\n p.incBallot() *3 \ngot = %#+v, \nwanted = %#+v", n.ballot.Counter, 3)
			}
		})
	}
}
