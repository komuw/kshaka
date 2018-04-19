/*
Package store provides sample implementations of kshaka's StableStore interface
That interface is the same as the one defined in hashicorp/raft
This means you can use any interface that satisfies that interface as the storage
mechanism for kshaka/CASPaxos,
eg github.com/hashicorp/raft-mdb and
   github.com/hashicorp/raft-boltdb
*/
package protocol

import (
	"errors"
	"sync"
)

// InmemStore implements the StableStore interface.
// It should  NEVER be used for production. It is used only for unit tests.
// Use the github.com/hashicorp/raft-mdb implementation instead.
// This InmemStore is based on the one defined in hashicorp/raft; with the difference been that
// this only satisfies the StableStore interface whereas the hashicorp/raft one also satisfies the LogStore interface.
// However CASPaxos(and kshaka by extension) unlike Raft and Multi-Paxos doesn’t use log replication.
type InmemStore struct {
	l     sync.RWMutex
	KV    map[string][]byte
	KVint map[string]uint64
}

// Set implements the StableStore interface.
func (i *InmemStore) Set(key []byte, val []byte) error {
	i.l.Lock()
	defer i.l.Unlock()
	i.KV[string(key)] = val
	return nil
}

// Get implements the StableStore interface.
func (i *InmemStore) Get(key []byte) ([]byte, error) {
	i.l.RLock()
	defer i.l.RUnlock()
	val := i.KV[string(key)]

	// see: https://github.com/hashicorp/raft-boltdb/blob/6e5ba93211eaf8d9a2ad7e41ffad8c6f160f9fe3/bolt_store.go#L241-L246
	// opened; hashicorp/raft/pull/286
	if val == nil {
		return nil, errors.New("not found")
	}
	return val, nil
}

// SetUint64 implements the StableStore interface.
func (i *InmemStore) SetUint64(key []byte, val uint64) error {
	i.l.Lock()
	defer i.l.Unlock()
	i.KVint[string(key)] = val
	return nil
}

// GetUint64 implements the StableStore interface.
func (i *InmemStore) GetUint64(key []byte) (uint64, error) {
	i.l.RLock()
	defer i.l.RUnlock()
	return i.KVint[string(key)], nil
}
