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
		{name: "increment ballot", n: NewNode(1, store)},
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

func TestNewNode(t *testing.T) {
	kv := map[string][]byte{"": []byte("")}
	store := &InmemStore{kv: kv}

	type args struct {
		store StableStore
		nodes []*Node
	}
	tests := []struct {
		name        string
		args        args
		numberNodes int
	}{
		{name: "no nodes supplied",
			args:        args{store: store},
			numberNodes: 1},
		{name: "1 node supplied",
			args:        args{store: store, nodes: []*Node{NewNode(1, store)}},
			numberNodes: 2},

		{name: "7 node supplied",
			args:        args{store: store, nodes: []*Node{NewNode(1, store), NewNode(2, store), NewNode(3, store), NewNode(4, store), NewNode(5, store), NewNode(6, store), NewNode(7, store)}},
			numberNodes: 8},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := NewNode(8, tt.args.store, tt.args.nodes...)
			numNodes := len(n.nodes)

			if numNodes != tt.numberNodes {
				t.Errorf("\n NewNode \nnumNodes= %#+v, \nwanted = %#+v", numNodes, tt.numberNodes)
			}

		})
	}
}
