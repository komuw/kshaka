package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type TransportProposeRequest struct {
	Key          []byte
	Val          []byte
	FunctionName string
}

func main() {

	key := []byte("name")
	val := []byte("Masta-Ace")

	propReq := TransportProposeRequest{Key: key, Val: val, FunctionName: "setFunc"}
	url := "http://" + "127.0.0.1" + ":" + "15003" + "/propose"
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
	fmt.Println("client Propose response body::", body, string(body))
}
