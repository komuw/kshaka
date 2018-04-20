package protocol

// StableStore is used to provide stable storage
// of key configurations to ensure safety.
// This interface is the same as the one defined in hashicorp/raft
type StableStore interface {
	Set(key []byte, val []byte) error
	// Get returns the value for key, or an empty byte slice if key was not found.
	Get(key []byte) ([]byte, error)
	SetUint64(key []byte, val uint64) error
	// GetUint64 returns the uint64 value for key, or 0 if key was not found.
	GetUint64(key []byte) (uint64, error)
}
