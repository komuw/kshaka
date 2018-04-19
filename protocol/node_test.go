package protocol

import (
	"testing"
)

func TestNode_incBallot(t *testing.T) {
	kv := map[string][]byte{"": []byte("")}
	store := &InmemStore{KV: kv}

	tests := []struct {
		name string
		n    *Node
	}{
		{name: "increment Ballot", n: NewNode(1, store)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := tt.n
			n.incBallot()
			n.incBallot()
			n.incBallot()

			if n.Ballot.Counter != 3 {
				t.Errorf("\n p.incBallot() *3 \ngot = %#+v, \nwanted = %#+v", n.Ballot.Counter, 3)
			}
		})
	}
}

func TestMingleNodes(t *testing.T) {
	kv := map[string][]byte{"": []byte("")}
	store := &InmemStore{KV: kv}
	tests := []struct {
		name        string
		nodes       []*Node
		numberNodes int
	}{
		{name: "1 node should include itself in n.nodes",
			nodes:       []*Node{NewNode(1, store)},
			numberNodes: 1},
		{name: "2 nodes supplied",
			nodes:       []*Node{NewNode(1, store), NewNode(2, store)},
			numberNodes: 2},
		{name: "5 nodes supplied",
			nodes:       []*Node{NewNode(1, store), NewNode(2, store), NewNode(3, store), NewNode(4, store), NewNode(5, store)},
			numberNodes: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MingleNodes(tt.nodes...)
			n := tt.nodes[0]
			numNodes := len(n.nodes)
			if numNodes != tt.numberNodes {
				t.Errorf("\n MingleNodes \nnumNodes= %#+v, \nwanted = %#+v", numNodes, tt.numberNodes)
			}

		})
	}
}

func TestMingleNodesMoreTimes(t *testing.T) {
	kv := map[string][]byte{"": []byte("")}
	store := &InmemStore{KV: kv}
	tests := []struct {
		name        string
		nodes       []*Node
		numberNodes int
	}{
		{name: "1 node should include itself in n.nodes",
			nodes:       []*Node{NewNode(1, store)},
			numberNodes: 1},
		{name: "2 nodes supplied",
			nodes:       []*Node{NewNode(1, store), NewNode(2, store)},
			numberNodes: 2},
		{name: "5 nodes supplied",
			nodes:       []*Node{NewNode(1, store), NewNode(2, store), NewNode(3, store), NewNode(4, store), NewNode(5, store)},
			numberNodes: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			MingleNodes(tt.nodes...)
			MingleNodes(tt.nodes...)
			MingleNodes(tt.nodes...)
			MingleNodes(tt.nodes...)

			n := tt.nodes[0]
			numNodes := len(n.nodes)
			if numNodes != tt.numberNodes {
				t.Errorf("\n MingleNodes \nnumNodes= %#+v, \nwanted = %#+v", numNodes, tt.numberNodes)
			}

		})
	}
}
