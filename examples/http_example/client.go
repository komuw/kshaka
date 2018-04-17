package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/komuw/kshaka"
)

func main() {
	var setFunc = func(val []byte) kshaka.ChangeFunction {
		return func(current []byte) ([]byte, error) {
			return val, nil
		}
	}
	key := []byte("name")
	val := []byte("Masta-Ace")

	propReq := kshaka.TransportProposeRequest{Key: key, ChangeFunc: setFunc(val)}
	url := "http://" + "127.0.0.1" + ":" + "15001" + "/propose"
	propReqJSON, err := json.Marshal(propReq)
	if err != nil {
		fmt.Printf("\n err: %+v \n", err)
		return
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(propReqJSON))
	if err != nil {
		fmt.Printf("\n err: %+v \n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second * 3}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("\n err: %+v \n", err)
		return
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("\n err: %+v \n", err)
		return
	}
	fmt.Println("client Propose response body::", string(body))
}
