package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/komuw/kshaka"
)

/*
HttpTransport provides a network based transport that can be
used to communicate with kshaka/CASPaxos on remote machines. It requires
an underlying stream layer to provide a stream abstraction, which can
be simple TCP, TLS, etc.
*/
type HttpTransport struct {
	NodeAddrress string
	NodePort     string
}

type TransportPrepareRequest struct {
	B   kshaka.Ballot
	Key []byte
}

func (ht *HttpTransport) TransportPrepare(b kshaka.Ballot, key []byte) (kshaka.AcceptorState, error) {
	fmt.Println("TransportPrepare called....")
	prepReq := TransportPrepareRequest{B: b, Key: key}
	url := "http://" + ht.NodeAddrress + ":" + ht.NodePort + "/prepare"
	prepReqJSON, err := json.Marshal(prepReq)
	if err != nil {
		return kshaka.AcceptorState{}, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(prepReqJSON))
	if err != nil {
		return kshaka.AcceptorState{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	// todo: ideally, client should be resused across multiple requests
	client := &http.Client{Timeout: time.Second * 3}
	resp, err := client.Do(req)
	if err != nil {
		return kshaka.AcceptorState{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return kshaka.AcceptorState{}, errors.New(fmt.Sprintf("url:%v returned http status:%v instead of status:%v", url, resp.StatusCode, http.StatusOK))
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return kshaka.AcceptorState{}, err
	}
	fmt.Println("TransportPrepare response body::", body, string(body))
	return kshaka.AcceptorState{}, nil
}

type TransportAcceptRequest struct {
	B     kshaka.Ballot
	Key   []byte
	State []byte
}

func (ht *HttpTransport) TransportAccept(b kshaka.Ballot, key []byte, state []byte) (kshaka.AcceptorState, error) {
	fmt.Println("TransportAccept called....")
	acceptReq := TransportAcceptRequest{B: b, Key: key, State: state}
	url := "http://" + ht.NodeAddrress + ":" + ht.NodePort + "/accept"
	acceptReqJSON, err := json.Marshal(acceptReq)
	if err != nil {
		return kshaka.AcceptorState{}, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(acceptReqJSON))
	if err != nil {
		return kshaka.AcceptorState{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second * 3}
	resp, err := client.Do(req)
	if err != nil {
		return kshaka.AcceptorState{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return kshaka.AcceptorState{}, errors.New(fmt.Sprintf("url:%v returned http status:%v instead of status:%v", url, resp.StatusCode, http.StatusOK))
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return kshaka.AcceptorState{}, err
	}
	fmt.Println("TransportAccept response body::", body)
	return kshaka.AcceptorState{}, nil
}
