package protocol

import (
	"reflect"
	"testing"
)

func TestPropose(t *testing.T) {
	kv := map[string][]byte{"": []byte("")}
	acceptorStore := &InmemStore{kv: kv}

	kv2 := map[string][]byte{"Bob": []byte("Marley")}
	acceptorStore2 := &InmemStore{kv: kv2}

	var readFunc ChangeFunction = func(current []byte) ([]byte, error) {
		return current, nil
	}

	var setFunc = func(val []byte) ChangeFunction {
		return func(current []byte) ([]byte, error) {
			return val, nil
		}

	}

	type args struct {
		key        []byte
		changeFunc ChangeFunction
	}
	tests := []struct {
		name    string
		n       *Node
		args    args
		want    []byte
		wantErr bool
	}{
		{name: "no acceptors",
			n:       &Node{ID: 1, Ballot: Ballot{}, acceptorStore: acceptorStore},
			args:    args{key: []byte("foo"), changeFunc: readFunc},
			want:    nil,
			wantErr: true,
		},
		{name: "two acceptors",
			n: &Node{ID: 1,
				Ballot: Ballot{},
				nodes: []*Node{NewNode(1, acceptorStore),
					NewNode(2, acceptorStore)},
				acceptorStore: acceptorStore},
			args:    args{key: []byte("foo"), changeFunc: readFunc},
			want:    nil,
			wantErr: true,
		},
		{name: "enough acceptors readFunc no key set",
			n: &Node{ID: 1,
				Ballot: Ballot{},
				nodes: []*Node{NewNode(1, acceptorStore),
					NewNode(2, acceptorStore),
					NewNode(3, acceptorStore),
					NewNode(4, acceptorStore)},
				acceptorStore: acceptorStore},
			args:    args{key: []byte("foo"), changeFunc: readFunc},
			want:    nil,
			wantErr: false,
		},
		{name: "enough acceptors readFunc with key set",
			n: &Node{ID: 1,
				Ballot: Ballot{},
				nodes: []*Node{NewNode(1, acceptorStore2),
					NewNode(2, acceptorStore2),
					NewNode(3, acceptorStore2),
					NewNode(4, acceptorStore2)},
				acceptorStore: acceptorStore2},
			args:    args{key: []byte("Bob"), changeFunc: readFunc},
			want:    []byte("Marley"),
			wantErr: false,
		},
		{name: "enough acceptors setFunc",
			n: &Node{ID: 1,
				Ballot: Ballot{},
				nodes: []*Node{NewNode(1, acceptorStore),
					NewNode(2, acceptorStore),
					NewNode(3, acceptorStore),
					NewNode(4, acceptorStore)},
				acceptorStore: acceptorStore},
			args:    args{key: []byte("stephen"), changeFunc: setFunc([]byte("hawking"))},
			want:    []byte("hawking"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// add transport
			transport := &InmemTransport{Node: tt.n}
			tt.n.AddTransport(transport)
			for _, n := range tt.n.nodes {
				n.AddTransport(transport)
			}

			newstate, err := tt.n.Propose(tt.args.key, tt.args.changeFunc)
			t.Logf("\nnewstate:%#+v, \nerr:%#+v", newstate, err)

			if (err != nil) != tt.wantErr {
				t.Errorf("\nproposer.Propose() \nerror = %v, \nwantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(newstate, tt.want) {
				t.Errorf("\nproposer.Propose() \ngot= %v, \nwant = %v", newstate, tt.want)
			}
		})
	}
}
