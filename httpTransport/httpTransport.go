/*
Package httpTransport provides a sample implementation of kshaka's transport interface
This implementation uses net/http to communicate between different kshaka Nodes.
*/
package httpTransport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/komuw/kshaka"
)

// HTTPtransport provides a http based transport that can be
// used to communicate with kshaka/CASPaxos on remote machines.
type HTTPtransport struct {
	NodeAddrress string
	NodePort     string
	ProposeURI   string
	PrepareURI   string
	AcceptURI    string
}

// PrepareRequest is the request sent during prepare phase
// specifically for the HTTPtransport
type PrepareRequest struct {
	B   kshaka.Ballot
	Key []byte
}

// TransportPrepare implements the Transport interface.
func (ht *HTTPtransport) TransportPrepare(b kshaka.Ballot, key []byte) (kshaka.AcceptorState, error) {
	acceptedState := kshaka.AcceptorState{}

	prepReq := PrepareRequest{B: b, Key: key}
	url := "http://" + ht.NodeAddrress + ":" + ht.NodePort + ht.PrepareURI
	prepReqJSON, err := json.Marshal(prepReq)
	if err != nil {
		return acceptedState, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(prepReqJSON))
	if err != nil {
		return acceptedState, err
	}
	req.Header.Set("Content-Type", "application/json")
	// todo: ideally, client should be resused across multiple requests
	client := &http.Client{Timeout: time.Second * 3}
	resp, err := client.Do(req)
	if err != nil {
		return acceptedState, err
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusOK {
		return acceptedState, fmt.Errorf("url:%v returned http status:%v instead of status:%v", url, resp.StatusCode, http.StatusOK)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return acceptedState, err
	}

	err = json.Unmarshal(body, &acceptedState)
	return acceptedState, err
}

// AcceptRequest is the request sent during accept phase
// specifically for the HTTPtransport
type AcceptRequest struct {
	B     kshaka.Ballot
	Key   []byte
	State []byte
}

// TransportAccept implements the Transport interface.
func (ht *HTTPtransport) TransportAccept(b kshaka.Ballot, key []byte, state []byte) (kshaka.AcceptorState, error) {
	acceptedState := kshaka.AcceptorState{}
	acceptReq := AcceptRequest{B: b, Key: key, State: state}
	url := "http://" + ht.NodeAddrress + ":" + ht.NodePort + ht.AcceptURI
	acceptReqJSON, err := json.Marshal(acceptReq)
	if err != nil {
		return acceptedState, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(acceptReqJSON))
	if err != nil {
		return acceptedState, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second * 3}
	resp, err := client.Do(req)
	if err != nil {
		return acceptedState, err
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode != http.StatusOK {
		return acceptedState, fmt.Errorf("url:%v returned http status:%v instead of status:%v", url, resp.StatusCode, http.StatusOK)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return acceptedState, err
	}

	err = json.Unmarshal(body, &acceptedState)
	return acceptedState, err
}
