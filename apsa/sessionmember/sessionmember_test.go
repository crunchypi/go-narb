package sessionmember

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"
)

var (
	NODECOUNT = 100
	ADDRS     = newWhitelist(NODECOUNT)

	SESSIONDURATION = time.Second * 2
	ARBITERDURATION = time.Second * 2
)

func newWhitelist(n int) []Addr {
	res := make([]Addr, n)
	for i := 0; i < n; i++ {
		res[i] = Addr{IP: "_", Port: fmt.Sprint(3000 + i)}
	}
	return res
}

/*
-------------------------------------------------------------------------------
Secion for test utils 1: pNetworkStatus. The tests defined in this file
will use a pseudo network type (defined further down), which is a collection
of SessionMember, to do actions on multiple SessionMember instances (such as
calling SessionMember.InitSession) in one go. Each of these calls will return
some status for all 'nodes' in the pseudo network, and pNetworkStatus will
be a convenient way of storing these statuses.
-------------------------------------------------------------------------------
*/

type pNetworkStatus map[Addr]StatusCode

// Check if all keys in pNetworkStatus are s.
func (pns *pNetworkStatus) uniformStatus(s StatusCode) bool {
	for _, status := range *pns {
		if status != s {
			return false
		}
	}
	return true
}

// Returns a pretty print string.
func (pns *pNetworkStatus) toStr() string {
	m := make(map[string]string)
	for addr, status := range *pns {
		m[addr.ToStr()] = status.ToStr()
	}
	s, _ := json.MarshalIndent(m, "", "\t")
	return string(s)
}

/*
-------------------------------------------------------------------------------
Secion for test utils 2: pNetwork (pseudo network). Some of the tests further
down in this file will require that all SessionMember instances that are used
are set into a state in one go, for instance doing SessionMember.InitSession.
It can get quite repetative to do these loops in all tests, so pNetwork solves
that problem.
-------------------------------------------------------------------------------
*/

type pNetwork map[Addr]*SessionMember

func newPseudoNetwork(whitelist []Addr) pNetwork {
	network := make(pNetwork)
	// Each SessionMember must have a way to contact other SessionMember
	// instances, specifically to call the Vote method. In a real scenario,
	// this func would do a network call. Here, it simply accessesi the
	// internal hashmap.
	f := func(remoteAddr Addr, sessionID ID) (Addr, ID, StatusCode) {
		sm, ok := network[remoteAddr]
		if !ok {
			panic("pseudo network 'connect' err for " + remoteAddr.ToStr())
		}
		return sm.Vote(sessionID)
	}
	for _, addr := range whitelist {
		cfg := NewSessionMemberConfig{
			LocalAddr:       addr,
			Whitelist:       whitelist,
			F:               Funcs{f},
			SessionDuration: SESSIONDURATION,
			ArbiterDuration: ARBITERDURATION,
		}
		network[addr] = NewSessionMember(cfg)
	}
	return network
}

// Simply call SessionMember.InitSession on all instances in pNetwork
// and return their collected status in pNetworkStatus (correlated keys).
func (p *pNetwork) initSession(sessionID ID, voteOpt []Addr) pNetworkStatus {
	r := make(pNetworkStatus)
	for address, sm := range *p {
		r[address] = sm.InitSession(sessionID, voteOpt)
	}
	return r
}

// Simply call SessionMember.CollectVotes on all instances in pNetwork
// and return their collected status in pNetworkStatus (correlated keys).
func (p *pNetwork) collectVotes(sessionID ID, voteOpt []Addr) pNetworkStatus {
	r := make(pNetworkStatus)
	for address, sm := range *p {
		status := sm.CollectVotes(sessionID, voteOpt)
		r[address] = status
	}
	return r
}

// Try to reach network consensus after n retries. id and whitelist args are
// passed along to all SessionMember instances in pNetwork when calling
// SessionMember.InitSession and SessionMember.CollectVotes. Return is a bool
// indicating consensus, an int specifying how many retries were needed and
// and err that is meant to be used for failure messages in each unit test.
// NOTE: This method assumes that all SessionMember isntances in pNetwork
// are in a state set by SessionMember.InitSession (call pNetwork.initSession
// could do this), if this is not the case, then the method will panic.
//
// Why: The arbiter consensus algo doesn't necessarily ensure consensus on the
// first try, as votes are done completely randomly. A few tests are conserned
// with whether or not a consensus can be reached at all, so this method reduces
// code duplication (this task in multiple tests).
func (p *pNetwork) retryVoteConsensus(id ID, whitelist []Addr, n int) (bool, int, error) {
	for i := 0; i < n; i++ {
		networkStatus := p.collectVotes(id, whitelist)

		// Handle case: network session not initialised, panic.
		for _, v := range networkStatus {
			if v == StatusNotInSession {
				s := "test setup fault. p.Network.retryVoteConsensus needs "
				s += "the network to be initialized with pNetwork.initSession."
				panic(s)
			}
		}

		// Handle case: no consensus, re-init network.
		if !networkStatus.uniformStatus(StatusOK) {
			networkStatus = p.initSession(id, whitelist)
			// Abort if re-init couldn't be done.
			if !networkStatus.uniformStatus(StatusOK) {
				s := "failed to re-init session. network:" + networkStatus.toStr()
				return false, i, errors.New(s)
			}
			continue
		}

		return true, i, nil
	}
	return false, n, nil
}

// Simply call SessionMember.Arbiter on all instances in pNetwork and
// return (1) bool indicating whether or not all arbiters gotten from
// nodes are the same and (2) collective status in pNetworkStatus form
// (correlated keys of that and pNetork).
func (p *pNetwork) arbiterConsensus() (bool, pNetworkStatus) {
	r := make(pNetworkStatus)
	consensus := true

	// Use the first node as comparison to all other.
	var sample *ArbiterData
	for address, sm := range *p {
		arbiter, status := sm.Arbiter()
		r[address] = status
		// First for comparison.
		if sample == nil {
			sample = &arbiter
			continue
		}
		if !arbiter.Addr.Comp((*sample).Addr) {
			consensus = false
		}
	}
	return consensus, r
}

/*
--------------------------------------------------------------------------------
Tests start here.


--------------------------------------------------------------------------------
*/

// Tests SessionMember.InitSession for all nodes in pNetwork.
func TestInit(t *testing.T) {
	network := newPseudoNetwork(ADDRS)
	id := newRandID(10)

	status := network.initSession(id, ADDRS)
	if !status.uniformStatus(StatusOK) {
		t.Fatalf("network status not StatusOK after first init:%v", status.toStr())
	}
}

// Tests SessionMember.CollectVotes for all nodes in pNetwork.
// Also validates the possible states after the call.
func TestCollectVotes(t *testing.T) {
	// Assumed working functionality.
	if ok := t.Run("SETUP 1", TestInit); !ok {
		t.Fatalf("test assumed 'TestInit' would work, it did not")
	}

	network := newPseudoNetwork(ADDRS)
	id := newRandID(10)

	network.initSession(id, ADDRS)
	// Verify the expected possible SessionMember states after this call.
	status := network.collectVotes(id, ADDRS)
	for k, v := range status {
		// brevity
		a := StatusOK
		b := StatusFailedVoteCollection
		c := StatusFailedVoteConsensus
		if v != a && v != b && v != c {
			t.Fatalf("unexpected status for %v: %v.", k, v.ToStr())
		}
	}
}

// Checks whether or not the network can reach a consensus.
func TestVoteConsensus(t *testing.T) {
	// Assumed working functionality.
	if ok := t.Run("SETUP 1", TestInit); !ok {
		t.Fatalf("test assumed 'TestInit' would work, it did not")
	}
	if ok := t.Run("SETUP 2", TestCollectVotes); !ok {
		t.Fatalf("test assumed 'TestCollectVotes' would work, it did not")
	}

	network := newPseudoNetwork(ADDRS)
	id := newRandID(10)

	// Return verified by SETUP 1.
	network.initSession(id, ADDRS)

	retries := 50
	consensus, foundAfter, err := network.retryVoteConsensus(id, ADDRS, retries)
	if err != nil {
		t.Fatalf("error while retrying vote consensus:\n%v", err)
	}
	if !consensus {
		t.Fatalf("failed to get vote consensus after %v retries", retries)
	}
	t.Logf("found vote consensus after %v retries", foundAfter)
}

// Checks whether or not all SessionMember instances in pNetwork give the
// same arbiter after a vote consensus is reached.
func TestArbiterConsensus(t *testing.T) {
	// Assumed working functionality.
	if ok := t.Run("SETUP 1", TestInit); !ok {
		t.Fatalf("test assumed 'TestInit' would work, it did not")
	}
	if ok := t.Run("SETUP 2", TestVoteConsensus); !ok {
		t.Fatalf("test assumed 'TestVoteConsensus' would work, it did not")
	}

	network := newPseudoNetwork(ADDRS)
	id := newRandID(10)

	// Return verified by SETUP 1.
	network.initSession(id, ADDRS)

	retries := 50
	// Return verified by SETUP 2.
	network.retryVoteConsensus(id, ADDRS, retries)

	consensus, status := network.arbiterConsensus()
	if !consensus {
		t.Fatalf("failed to get arbiter consensus:%v\n", status.toStr())
	}
}

func TestArbiterExpiredReinit(t *testing.T) {
	// Assumed working functionality.
	if ok := t.Run("SETUP 1", TestInit); !ok {
		t.Fatalf("test assumed 'TestInit' would work, it did not")
	}
	if ok := t.Run("SETUP 2", TestVoteConsensus); !ok {
		t.Fatalf("test assumed 'TestVoteConsensus' would work, it did not")
	}
	if ok := t.Run("SETUP 3", TestArbiterConsensus); !ok {
		t.Fatalf("test assumed 'TestArbiterConsensus' would work, it did not")
	}

	network := newPseudoNetwork(ADDRS)
	id := newRandID(10)

	// Return verified by SETUP 1.
	network.initSession(id, ADDRS)

	// Force arbiter, return verified by SETUP 2 & 3.
	retries := 20
	network.retryVoteConsensus(id, ADDRS, retries)
	network.arbiterConsensus()

	// Expire..
	time.Sleep(ARBITERDURATION)
	_, status := network.arbiterConsensus()
	if !status.uniformStatus(StatusArbiterExpired) {
		t.Fatalf("arbiter didn't expire: %v", status.toStr())
	}

	// Check that arbiter can be set again.
	status = network.initSession(id, ADDRS)
	if !status.uniformStatus(StatusOK) {
		t.Fatalf("no network re-init after arbiter expired: %v", status.toStr())
	}

	// Try force arbiter again.
	network.retryVoteConsensus(id, ADDRS, retries)
	consensus, status := network.arbiterConsensus()
	if !consensus {
		t.Fatalf("failed to get arbiter consensus after expiration:%v\n", status.toStr())
	}
}
