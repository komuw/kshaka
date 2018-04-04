package kshaka

/*
ChangeFunction is the function that clients send to proposers.
The function takes the current state as an argument and yields the new value(state) as a result.

An example ChangeFunction is given below:

	var readFunc ChangeFunction = func(key []byte, current []byte) ([]byte, error) {
		return current, nil
	}

A client can send the above change function to a proposer, when the client wants to read the value stored
at a key named foo. The proposer will apply that function to the current state(ie to the current value stored at key foo) and return
the new value stored at that key and an error.
*/
type ChangeFunction func(key []byte, currentState []byte) ([]byte, error)
