package kshaka

import (
	"reflect"
	"testing"
)

func Test_proposer_Propose(t *testing.T) {
	kv2 := map[string][]byte{"": []byte("")}
	acceptorStore := &InmemStore{kv: kv2}

	var readFunc ChangeFunction = func(key []byte, current []byte) ([]byte, error) {
		return current, nil
	}

	var setFunc = func(key []byte, val []byte) ChangeFunction {
		return func(key []byte, current []byte) ([]byte, error) {
			return val, nil
		}
	}

	type args struct {
		key        []byte
		changeFunc ChangeFunction
	}
	tests := []struct {
		name    string
		p       proposer
		args    args
		want    []byte
		wantErr bool
	}{
		{name: "no acceptors",
			p:       proposer{id: 1, ballot: ballot{Counter: 1, ProposerID: 1}},
			args:    args{key: []byte("foo"), changeFunc: readFunc},
			want:    nil,
			wantErr: true,
		},
		{name: "two acceptors",
			p:       proposer{id: 1, ballot: ballot{Counter: 1, ProposerID: 1}, acceptors: []*acceptor{&acceptor{id: 1}, &acceptor{id: 2}}},
			args:    args{key: []byte("foo"), changeFunc: readFunc},
			want:    nil,
			wantErr: true,
		},
		{name: "enough acceptors readFunc",
			p:       proposer{id: 1, ballot: ballot{Counter: 1, ProposerID: 1}, acceptors: []*acceptor{&acceptor{id: 1}, &acceptor{id: 2}, &acceptor{id: 3}, &acceptor{id: 4}}},
			args:    args{key: []byte("foo"), changeFunc: readFunc},
			want:    nil,
			wantErr: false,
		},
		{name: "enough acceptors setFunc",
			p:       proposer{id: 1, ballot: ballot{Counter: 1, ProposerID: 1}, acceptors: []*acceptor{&acceptor{id: 1}, &acceptor{id: 2}, &acceptor{id: 3}, &acceptor{id: 4}}},
			args:    args{key: []byte("foo"), changeFunc: setFunc([]byte("stephen"), []byte("hawking"))},
			want:    []byte("hawking"),
			wantErr: false,
		},

		{name: "enough acceptors setFunc. acceptor with own stores",
			p:       proposer{id: 1, ballot: ballot{Counter: 1, ProposerID: 1}, acceptors: []*acceptor{&acceptor{id: 1, stateStore: acceptorStore}, &acceptor{id: 2, stateStore: acceptorStore}, &acceptor{id: 3, stateStore: acceptorStore}, &acceptor{id: 4, stateStore: acceptorStore}}},
			args:    args{key: []byte("foo"), changeFunc: setFunc([]byte("bob"), []byte("marley"))},
			want:    []byte("marley"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &tt.p
			newstate, err := p.Propose(tt.args.key, tt.args.changeFunc)
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
