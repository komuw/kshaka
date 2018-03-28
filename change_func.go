package kshaka

import "fmt"

type ChangeFunction func(currentState []byte) []byte

func readpo(x []byte) ChangeFunction {
	fmt.Println("x", string(x))
	return func(current []byte) []byte {
		fmt.Println("current", string(current))
		return current
	}
}

type StableStore interface {
	Set(key []byte, val []byte) error
	// Get returns the value for key, or an empty byte slice if key was not found.
	Get(key []byte) ([]byte, error)
	SetUint64(key []byte, val uint64) error
	// GetUint64 returns the uint64 value for key, or 0 if key was not found.
	GetUint64(key []byte) (uint64, error)
}

type ChangeF func(currentState StableStore) StableStore

func read(x []byte) ChangeF {
	fmt.Println("x", string(x))

	return func(current StableStore) StableStore {
		fmt.Println("current", current)
		return current
	}
}
