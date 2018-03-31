package kshaka

/*
ChangeFunction is the function that clients send to proposers.
The function takes the current state(StableStore) as an argument and yields the value  as a result.

An example ChangeFunction is given below:
var readFunc ChangeFunction = func(key []byte, current StableStore) ([]byte, error) {
	value, err := current.Get(key)
	return value, err
}

A client can send the above change function to a proposer, when the client wants to read the value stored
at a key named foo. The proposer will apply that function to the current state of the StableStore and return
the value stored at that key and an error.

kv := map[string][]byte{"foo": []byte("bar")}
m := &InmemStore{kv: kv}
key := []byte("foo")
ans, err := readFunc(key, m)
fmt.Printf("\n\n ans:%#+v err:%+#v\n", string(ans), err)
// ans:"bar" err:<nil>

*/
//type ChangeFunction func(currentState StableStore) ([]byte, error)
type ChangeFunction func(key []byte, currentState StableStore) ([]byte, error)
