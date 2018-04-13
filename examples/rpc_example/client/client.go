package main

import (
	"fmt"
	"net/rpc"

	"github.com/komuw/kshaka"
)

func main() {
	client, err := rpc.DialHTTP("tcp", "localhost"+":12000")
	if err != nil {
		fmt.Println()
		fmt.Println("client error 1::", err)
		fmt.Println()
	}

	var setFunc = func(val []byte) kshaka.ChangeFunction {
		return func(current []byte) ([]byte, error) {
			return val, nil
		}
	}

	key := []byte("name")
	val := []byte("Masta-Ace")

	args := kshaka.ProposeArgs{Key: key, ChangeFunc: setFunc(val)}

	var reply []byte

	// makes a Synchronous call. there are also async versions of this??
	err = client.Call("Node.ProposeRPC", args, &reply)
	if err != nil {
		fmt.Println()
		fmt.Println("client error 2::", err)
		fmt.Println()
	}
	fmt.Println("reply::", reply)

}
