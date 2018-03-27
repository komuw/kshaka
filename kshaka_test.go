package kshaka

import (
	"fmt"
	"testing"
)

func Test_proposer_addAcceptor(t *testing.T) {
	tt := []struct {
		p           *proposer
		acceptors   []acceptor
		expectedErr error
	}{
		{&proposer{}, []acceptor{acceptor{}}, nil},
		{&proposer{}, []acceptor{acceptor{}, acceptor{id: 100, acceptedValue: []byte("cool")}}, nil},
		{&proposer{}, []acceptor{acceptor{}, acceptor{id: 100, acceptedValue: []byte("cool")}, acceptor{id: 123, acceptedValue: []byte("okay"), acceptedBallot: ballot{counter: 3, proposerID: 7}}}, nil},
	}

	for _, v := range tt {
		var err error
		for _, val := range v.acceptors {
			var nonShadow = val
			e := v.p.addAcceptor(&nonShadow)
			err = e
		}
		fmt.Printf("\nproposer: %+v\n", v.p)
		if err != nil {
			t.Errorf("\nCalled p.addAcceptor(%#+v) \ngot %s \nwanted %#+v", v.acceptors, err, v.expectedErr)
		}
	}

}

func Test_acceptor_prepare(t *testing.T) {
	ok := []struct {
		acceptor             acceptor
		acceptedBallotCouter uint64
		acceptedValue        []byte
		expectedErr          error
	}{
		{acceptor{}, 0, []byte{}, nil},
		{acceptor{id: 100, acceptedValue: []byte("cool")}, 0, []byte{}, nil},
	}

	conflict := []struct {
		acceptor             acceptor
		acceptedBallotCouter uint64
		acceptedValue        []byte
	}{
		{acceptor{id: 123, acceptedValue: []byte("okay"), acceptedBallot: ballot{counter: 8, proposerID: 7}}, 8, []byte("okay")},
	}

	b := ballot{counter: 1, proposerID: 1}

	for _, v := range ok {
		acceptedBallot, acceptedValue, err := v.acceptor.prepare(b)
		fmt.Printf("%#+v", acceptedValue)

		if err != v.expectedErr {
			t.Errorf("\nCalled acceptor.prepare(%#+v) \ngot %#+v \nwanted %#+v", b, err, v.expectedErr)
		}

		if acceptedBallot.counter != v.acceptedBallotCouter {
			t.Errorf("\nCalled acceptor.prepare(%#+v) \ngot %#+v \nwanted %#+v", b, acceptedBallot.counter, v.acceptedBallotCouter)
		}
	}

	for _, v := range conflict {
		acceptedBallot, acceptedValue, err := v.acceptor.prepare(b)
		fmt.Printf("%#+v", acceptedValue)

		if err == nil {
			t.Errorf("\nCalled acceptor.prepare(%#+v) \ngot %#+v \nwanted %#+v", b, err, "prepareError")
		}

		if acceptedBallot.counter != v.acceptedBallotCouter {
			t.Errorf("\nCalled acceptor.prepare(%#+v) \ngot %#+v \nwanted %#+v", b, acceptedBallot.counter, v.acceptedBallotCouter)
		}
	}
}
