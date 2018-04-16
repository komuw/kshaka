package kshaka

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type Transport interface {
	// Propose is the method that clients call when they want to submit
	// the f change function to a proposer.
	// It takes the key whose value you want to apply the ChangeFunction to
	// and also the ChangeFunction that will be applied to the value(contents) of that key.
	TransportPropose(key []byte, changeFunc ChangeFunction) ([]byte, error)

	TransportPrepare(b ballot, key []byte) (AcceptorState, error)
	TransportAccept(b ballot, key []byte, state []byte) (AcceptorState, error)
}

// InmemTransport Implements the Transport interface, to allow kshaka/CASPaxos to be
// tested in-memory without going over a network.
type InmemTransport struct {
	NodeAddrress string
	NodePort     string
	Node         *Node
}

func (it *InmemTransport) TransportPropose(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	return it.Node.propose(key, changeFunc)
}
func (it *InmemTransport) TransportPrepare(b ballot, key []byte) (AcceptorState, error) {
	return it.Node.prepare(b, key)
}
func (it *InmemTransport) TransportAccept(b ballot, key []byte, state []byte) (AcceptorState, error) {
	return it.Node.accept(b, key, state)
}

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

type TransportProposeRequest struct {
	Key        []byte
	ChangeFunc ChangeFunction
}

func (ht *HttpTransport) TransportPropose(key []byte, changeFunc ChangeFunction) ([]byte, error) {
	propReq := TransportProposeRequest{Key: key, ChangeFunc: changeFunc}
	url := "http://" + ht.NodeAddrress + ":" + ht.NodePort + "/propose"
	propReqJSON, err := json.Marshal(propReq)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(propReqJSON))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second * 3}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	fmt.Println("TransportPropose response body::", body)
	return nil, nil
}

func (ht *HttpTransport) TransportPrepare(b ballot, key []byte) (AcceptorState, error) {
	type prepareRequest struct {
		B   ballot
		Key []byte
	}
	prepReq := prepareRequest{B: b, Key: key}
	url := "http://" + ht.NodeAddrress + ":" + ht.NodePort + "/prepare"
	prepReqJSON, err := json.Marshal(prepReq)
	if err != nil {
		return AcceptorState{}, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(prepReqJSON))
	if err != nil {
		return AcceptorState{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	// todo: ideally, client should be resused across multiple requests
	client := &http.Client{Timeout: time.Second * 3}
	resp, err := client.Do(req)
	if err != nil {
		return AcceptorState{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return AcceptorState{}, err
	}
	fmt.Println("TransportPrepare response body::", body)
	return AcceptorState{}, nil
}

func (ht *HttpTransport) TransportAccept(b ballot, key []byte, state []byte) (AcceptorState, error) {
	type acceptRequest struct {
		B     ballot
		Key   []byte
		State []byte
	}
	acceptReq := acceptRequest{B: b, Key: key, State: state}
	url := "http://" + ht.NodeAddrress + ":" + ht.NodePort + "/accept"
	acceptReqJSON, err := json.Marshal(acceptReq)
	if err != nil {
		return AcceptorState{}, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(acceptReqJSON))
	if err != nil {
		return AcceptorState{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second * 3}
	resp, err := client.Do(req)
	if err != nil {
		return AcceptorState{}, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return AcceptorState{}, err
	}
	fmt.Println("TransportAccept response body::", body)
	return AcceptorState{}, nil
}
