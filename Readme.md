# Kshaka

Kshaka is a Go implementation of the [CASPaxos](https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf) consensus protocol.                              
It's name is derived from Kenyan hip hop group, Kalamashaka.                
>CASPaxos is a replicated state machine (RSM) protocol. Unlike Raft and Multi-Paxos, it doesn’t use leader election and log replication, thus avoiding associated complexity.                   
Its symmetric peer-to-peer approach achieves optimal commit latency in wide-area networks and doesn’t cause transient unavailability when any ⌊N−1⌋ of N nodes crash." - [The CASPaxos whitepaper](https://github.com/rystsov/caspaxos/blob/master/latex/caspaxos.pdf)         


# Installation
- todo          


# Usage
- todo               


# System design

### 1. Intro:           
- Clients initiate a request by communicating with a proposer; clients may be stateless, the system may have arbitrary numbers of clients.               
- Proposers perform the initialization by communicating with acceptors. 
Proposers keep minimal state needed to generate unique increasing update IDs (ballot numbers), the system may have arbitrary numbers of proposers.        
- Acceptors store the acceptedv alue; the system should have 2F+1 acceptors to tolerate F failures.


- It’s convenient to use tuples as ballot numbers. 
To generate it a proposer combines its numerical ID with a local increasing counter: (counter, ID). 
To compare ballot tuples, we should compare the first component of the tuples and use ID only as a tiebreaker.
- When a proposer receives a conflicting message from an acceptor, it should fast-forward its counter to avoid a conflict in the future.

- Clients change its value by submitting side-effect free functions which take the current state as an argument and yield new as a result. 
Out of the concurrent requests only one can succeed;  we should acquire a lock:: https://github.com/gryadka/js/blob/dfc6ed6f7580c895a9db44d06756a3dd637e47f6/core/src/Proposer.js#L47-L48 

### 2. Algo:            

A. Prepare phase
- A client submits the f change function to a proposer.
- The proposer generates a ballot number, B, and sends ”prepare” messages containing that number(and it's ID) to the acceptors.
- Acceptor returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
Persists the ballot number as a promise and returns a confirmation either with an empty value (if it hasn’t accepted any value yet) or with a tuple of an accepted value and its ballot number.
- Proposer waits for the F + 1 confirmations.               

B. Accept phase
- If they(prepare replies from acceptors) all contain the empty value, then the proposer defines the current state as ∅ otherwise it picks the value of the tuple with the highest ballot number.             
- Proposer applies the f function to the current state and sends the result, new state, along with the generated ballot number B (an ”accept” message) to the acceptors.
- Accept returns a conflict if it already saw a greater ballot number, it also submits the ballot and accepted value it has.
Erases the promise, marks the received tuple (ballot number, value) as the accepted value and returns a confirmation        

C. End
- Proposer waits for the F + 1 confirmations.
- Proposer returns the new state to the client.       

### 3. Cluster membership change
- todo                 

### 4. Deleting record/s
- todo         

### 5. Optimizations
- todo          
