package kshaka

import "testing"

func Test_proposer_sendPrepare(t *testing.T) {
	type fields struct {
		id        uint64
		state     []byte
		ballot    ballot
		acceptors []*acceptor
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{"no acceptors", fields{id: 1, state: []byte("cool"), ballot: ballot{counter: 1, proposerID: 1}}, true},

		{"less acceptors",
			fields{
				id:     1,
				state:  []byte("cool"),
				ballot: ballot{counter: 1, proposerID: 1},
				acceptors: []*acceptor{
					&acceptor{id: 1, acceptedState: acceptorState{}},
					&acceptor{id: 3, acceptedState: acceptorState{acceptedBallot: ballot{}, acceptedValue: []byte("myValue")}},
				}},
			true},

		{"enough acceptors",
			fields{
				id:     1,
				state:  []byte("cool"),
				ballot: ballot{counter: 1, proposerID: 1},
				acceptors: []*acceptor{
					&acceptor{id: 1, acceptedState: acceptorState{}},
					&acceptor{id: 3, acceptedState: acceptorState{acceptedBallot: ballot{}, acceptedValue: []byte("myValue")}},
					&acceptor{id: 4, acceptedState: acceptorState{acceptedBallot: ballot{}, acceptedValue: []byte("myValue2")}},
					&acceptor{id: 5, acceptedState: acceptorState{acceptedBallot: ballot{counter: 2, proposerID: 2}, acceptedValue: []byte("myValue3")}},
				}},
			false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &proposer{
				id:        tt.fields.id,
				state:     tt.fields.state,
				ballot:    tt.fields.ballot,
				acceptors: tt.fields.acceptors,
			}
			if err := p.sendPrepare(); (err != nil) != tt.wantErr {
				t.Errorf("proposer.sendPrepare() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
