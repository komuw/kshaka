package kshaka

import (
	"fmt"
	"reflect"
	"testing"
)

func Test_proposer_addAcceptor(t *testing.T) {
	tt := []struct {
		p           *proposer
		expectedErr error
	}{
		{&proposer{}, nil},
	}
	a := acceptor{}
	for _, v := range tt {
		err := v.p.addAcceptor(a)
		fmt.Printf("%#+v", v.p)
		if err != nil {
			t.Errorf("\nCalled p.addAcceptor(%#+v) \ngot %s \nwanted %#+v", a, err, v.expectedErr)
		}
	}
}

func Test_acceptor_prepare(t *testing.T) {
	type fields struct {
		id             uint64
		acceptedBallot ballot
		acceptedValue  []byte
	}
	type args struct {
		b ballot
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    ballot
		want1   []byte
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &acceptor{
				id:             tt.fields.id,
				acceptedBallot: tt.fields.acceptedBallot,
				acceptedValue:  tt.fields.acceptedValue,
			}
			got, got1, err := a.prepare(tt.args.b)
			if (err != nil) != tt.wantErr {
				t.Errorf("acceptor.prepare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("acceptor.prepare() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("acceptor.prepare() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
