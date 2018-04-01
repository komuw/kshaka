package kshaka

import (
	"reflect"
	"testing"
)

func Test_acceptor_accept(t *testing.T) {
	kv := map[string][]byte{"foo": []byte("bar")}
	m := &InmemStore{kv: kv}

	type args struct {
		b     ballot
		key   []byte
		value []byte
	}
	tests := []struct {
		name               string
		a                  acceptor
		args               args
		wantedState        acceptorState
		wantedConfirmation bool
		wantErr            bool
	}{
		{name: "unable to get state",
			a:                  acceptor{id: 1, stateStore: m},
			args:               args{b: ballot{Counter: 1, ProposerID: 1}, key: []byte("notFound"), value: []byte("bar")},
			wantedState:        acceptorState{},
			wantedConfirmation: false,
			wantErr:            true,
		},
	}
	// TODO: add more testcases
	// TODO: fix mutex copy
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &tt.a
			acceptedState, confirmed, err := a.accept(tt.args.b, tt.args.key, tt.args.value)
			t.Logf("\nerr:%#+v", err)
			if (err != nil) != tt.wantErr {
				t.Errorf("\nacceptor.accept() \nerror = %#+v, \nwantErr = %#+v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(acceptedState, tt.wantedState) {
				t.Errorf("\nacceptor.accept() \nacceptedState = %#+v, \nwantedState = %#+v", acceptedState, tt.wantedState)
			}
			if confirmed != tt.wantedConfirmation {
				t.Errorf("\nacceptor.accept() \nconfirmed = %#+v, \nwantedConfirmation = %#+v", confirmed, tt.wantedConfirmation)
			}
		})
	}
}
