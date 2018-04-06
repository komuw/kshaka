package kshaka

import (
	"reflect"
	"testing"
)

func Test_newProposerAcceptor(t *testing.T) {
	tests := []struct {
		name string
		want *proposerAcceptor
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := newProposerAcceptor(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("newProposerAcceptor() = %v, want %v", got, tt.want)
			}
		})
	}
}
