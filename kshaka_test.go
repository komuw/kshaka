package kshaka

import (
	"bytes"
	"errors"
	"reflect"
	"sync"
	"testing"
)

type ErrInmemStore struct {
	l     sync.RWMutex
	kv    map[string][]byte
	kvInt map[string]uint64
}

// Set implements the StableStore interface.
func (i *ErrInmemStore) Set(key []byte, val []byte) error {
	i.l.Lock()
	defer i.l.Unlock()
	i.kv[string(key)] = val
	if bytes.Equal(key, []byte("unable to set state")) {
		return errors.New("Set error")
	}
	return nil
}

// Get implements the StableStore interface.
func (i *ErrInmemStore) Get(key []byte) ([]byte, error) {
	i.l.RLock()
	defer i.l.RUnlock()
	if bytes.Equal(key, []byte("unable to get state")) {
		return i.kv[string(key)], errors.New("Get error")
	} else if bytes.Equal(key, []byte("unable to get acceptedBallot")) {
		return i.kv[string(key)], errors.New("Get error")
	}
	return i.kv[string(key)], nil
}

// SetUint64 implements the StableStore interface.
func (i *ErrInmemStore) SetUint64(key []byte, val uint64) error {
	i.l.Lock()
	defer i.l.Unlock()
	i.kvInt[string(key)] = val
	return errors.New("SetUint64 error")
}

// GetUint64 implements the StableStore interface.
func (i *ErrInmemStore) GetUint64(key []byte) (uint64, error) {
	i.l.RLock()
	defer i.l.RUnlock()
	return i.kvInt[string(key)], errors.New("GetUint64 error")
}

func Test_acceptor_prepare(t *testing.T) {
	kv := map[string][]byte{"foo": []byte("bar")}
	m := &ErrInmemStore{kv: kv}

	type args struct {
		b   ballot
		key []byte
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
			args:               args{b: ballot{Counter: 1, ProposerID: 1}, key: []byte("unable to get state")},
			wantedState:        acceptorState{},
			wantedConfirmation: false,
			wantErr:            true,
		},
		{name: "unable to get acceptedBallot",
			a:                  acceptor{id: 1, stateStore: m},
			args:               args{b: ballot{Counter: 1, ProposerID: 1}, key: []byte("unable to get acceptedBallot")},
			wantedState:        acceptorState{},
			wantedConfirmation: false,
			wantErr:            true,
		},
		{name: "no acceptedBallot",
			a:                  acceptor{id: 1, stateStore: m},
			args:               args{b: ballot{Counter: 1, ProposerID: 1}, key: []byte("no acceptedBallot")},
			wantedState:        acceptorState{acceptedBallot: ballot{Counter: 1, ProposerID: 1}},
			wantedConfirmation: true,
			wantErr:            false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &tt.a
			acceptedState, confirmed, err := a.prepare(tt.args.b, tt.args.key)
			t.Logf("\nerror got:%#+v", err)
			t.Logf("\nacceptedState:%#+v. confirmed:%#+v", acceptedState, confirmed)

			if (err != nil) != tt.wantErr {
				t.Errorf("\nacceptor.prepare() \nerror = %v, \nwantErr = %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(acceptedState, tt.wantedState) {
				t.Errorf("\nacceptor.prepare() \nacceptedState = %v, \nwantedState = %v", acceptedState, tt.wantedState)
			}
			if confirmed != tt.wantedConfirmation {
				t.Errorf("\nacceptor.prepare() \nconfirmed = %v, \nwantedConfirmation = %v", confirmed, tt.wantedConfirmation)
			}
		})
	}
}

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
			args:               args{b: ballot{Counter: 1, ProposerID: 1}, key: []byte("foo"), value: []byte("bar")},
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
			t.Logf("\nerror got:%#+v", err)
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
