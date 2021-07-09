package rpc

import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/crunchypi/go-narb/apsa/common"
	"github.com/crunchypi/go-narb/apsa/sessionmember"
)

var (
	nodes   = 20
	addrs   []Addr
	network *tNetwork
)

var (
	// False will use addresses made with 'nedAddrsLinear',
	// True will use 'netAddrsRandomWithCheck'.
	randomPorts = false
	portsUpper  = 9000
	portsLower  = 3000
)

// These should be high if 'nodes' is high.
var (
	sessionDuration = time.Second * time.Duration(nodes)
	arbiterDuration = time.Second * time.Duration(nodes)
)

func reinit() {
	if network != nil {
		network.stop()
	}
	addrs = netAddrs(nodes)
	n := newNetwork(addrs)
	network = &n
}

func init() {
	reinit()
}

/*
-------------------------------------------------------------------------------
Section for utils: contains a type representing a network, with a few methods
which does all operations in ./client.go for all 'nodes'. This is mainly to
reduce repetition in proper test funcs.
-------------------------------------------------------------------------------
*/

func netAddrsLinear(n int) []Addr {
	res := make([]Addr, n)
	for i := 0; i < n; i++ {
		res[i] = Addr{IP: "localhost", Port: fmt.Sprint(portsLower + i)}
	}
	return res
}

func netAddrsRandomWithCheck(n int) []Addr {
	res := make([]Addr, 0, n)
	startAt := portsLower
	endAt := portsUpper

	i := 0
	for {
		if len(res) >= n || i > endAt {
			break
		}
		rand.Seed(time.Now().UnixNano())
		port := rand.Intn(endAt-startAt) + startAt

		addr := Addr{IP: "localhost", Port: fmt.Sprintf("%v", port)}
		ln, err := net.Listen("tcp", addr.ToStr())
		if err == nil {
			res = append(res, addr)
		}

		ln.Close()
		i++
	}
	return res
}

func netAddrs(n int) []Addr {
	if randomPorts {
		return netAddrsRandomWithCheck(n)
	}
	return netAddrsLinear(n)
}

func newSessionMember(localAddr Addr, whitelist []Addr) sessionMember {
	f := sessionmember.Funcs{RemoteVoteFunc: RemoteVoteFunc()}
	cfg := sessionmember.NewSessionMemberConfig{
		LocalAddr:       localAddr,
		Whitelist:       whitelist,
		F:               f,
		SessionDuration: sessionDuration,
		ArbiterDuration: arbiterDuration,
	}
	return sessionmember.NewSessionMember(cfg)
}

type tNetwork struct {
	nodes     map[Addr]*ArbiterServer
	stopFuncs map[Addr]func()
}

func newNetwork(addrs []Addr) tNetwork {
	r := tNetwork{
		nodes:     make(map[Addr]*ArbiterServer, len(addrs)),
		stopFuncs: make(map[Addr]func(), len(addrs)),
	}

	for _, addr := range addrs {
		s := NewArbiterServer(newSessionMember(addr, addrs))
		r.nodes[addr] = s

		f, err := StartListen(s, addr)
		if err != nil {
			panic("setup fail: couldn't start server with addr " + addr.ToStr())
		}
		r.stopFuncs[addr] = f
	}
	return r
}

func (t *tNetwork) resetSessionMember() {
	for addr, server := range t.nodes {
		server.ArbiterSessionMember = newSessionMember(addr, addrs)
	}
}

func (t *tNetwork) stop() {
	for _, f := range *&t.stopFuncs {
		f()
	}
}

// ArbiterClient.InitSession -> ArbiterServer.InitSession for nodes in tNetwork.
func (t *tNetwork) initSession(sessionID ID, voteOpt []Addr) error {
	for addr := range t.nodes {
		var err error
		status := ArbiterClient(addr, &err).InitSession(sessionID, voteOpt)
		if err != nil {
			s := fmt.Sprintf("unexpected net err for '%v': %v", addr.ToStr(), err)
			return errors.New(s)
		}
		if status != StatusOK {
			s := "unexpected status from"
			s = fmt.Sprintf("%s '%v': %v", s, addr.ToStr(), status.ToStr())
			return errors.New(s)
		}
	}
	return nil
}

// ArbiterClient.Vote -> ArbiterServer.Vote for all nodes in tNetwork.
func (t *tNetwork) vote(sessionID ID) error {
	for addr := range t.nodes {
		var err error
		_, _, status := ArbiterClient(addr, &err).Vote(sessionID)
		if err != nil {
			s := fmt.Sprintf("network err for '%v': %v", addr.ToStr(), err)
			return errors.New(s)
		}
		if status != StatusOK {
			s := "unexpected status from"
			s = fmt.Sprintf("%v '%v': %v", s, addr.ToStr(), status.ToStr())
			return errors.New(s)
		}
	}
	return nil
}

// ArbiterClient.CollectVotes -> ArbiterServer.CollectVotes for all nodes in tNetwork.
func (t *tNetwork) collectVotes(sessionID ID, voteOpt []Addr) (bool, error) {
	consensus := false
	for addr := range t.nodes {
		var err error
		status := ArbiterClient(addr, &err).CollectVotes(sessionID, voteOpt)
		if err != nil {
			s := fmt.Sprintf("network err for '%v': %v", addr.ToStr(), err)
			return false, errors.New(s)
		}

		// Abbreviations
		a := StatusOK
		b := StatusFailedVoteConsensus
		c := StatusFailedVoteCollection
		if status != a && status != b && status != c {
			s := "unexpected status for"
			s = fmt.Sprintf("%v '%v': %v", s, addr.ToStr(), status.ToStr())
			return false, errors.New(s)
		}
		if status == StatusOK {
			// Not doing a break because that would leave the network in
			// a partial/inconsistent state.
			consensus = true
		}
	}
	return consensus, nil
}

/*
--------------------------------------------------------------------------------
Section for tests. These will validate the functionality of both server.go
and client.go; naming will be based on client.go. This will be mainly done
on collections for server instances in tNetwork (should be defined further up).

--------------------------------------------------------------------------------
*/

// Test 'Ping' method for all nodes in tNetwork.
func TestPing(t *testing.T) {
	for addr := range network.nodes {
		var err error
		ok := ArbiterClient(addr, &err).Ping()
		if err != nil {
			t.Fatalf("unexpected ping err for '%v': %v", addr.ToStr(), err)
		}
		if !ok {
			t.Fatalf("unexpected 'false' resp from '%v'", addr.ToStr())
		}
	}
}

// Test 'InitSession' method for all nodes in tNetwork.
func TestInitSession(t *testing.T) {
	network.resetSessionMember()
	id := common.NewRandID(10)

	if err := network.initSession(id, addrs); err != nil {
		t.Fatal(err)
	}
}

// Test 'Vote' method for all nodes in tNetwork.
func TestVote(t *testing.T) {
	// Verify that assumed functionality works.
	if ok := t.Run("SETUP 1", TestInitSession); !ok {
		t.Fatalf("Expected TestInitSession to work, it did not")
	}

	network.resetSessionMember()
	id := common.NewRandID(10)

	if err := network.initSession(id, addrs); err != nil {
		t.Fatal(err)
	}
	if err := network.vote(id); err != nil {
		t.Fatal(err)
	}
}

// Test 'CollectVotes' for all nodes in tNetwork.
func TestCollectVotes(t *testing.T) {
	// Verify that assumed functionality works.
	if ok := t.Run("SETUP 1", TestInitSession); !ok {
		t.Fatalf("Expected TestInitSession to work, it did not")
	}
	if ok := t.Run("SETUP 2", TestVote); !ok {
		t.Fatalf("Expected TestVote to work, it did not")
	}

	network.resetSessionMember()
	id := common.NewRandID(10)

	if err := network.initSession(id, addrs); err != nil {
		t.Fatal(err)
	}
	if _, err := network.collectVotes(id, addrs); err != nil {
		t.Fatal(err)
	}
}

// This test tries to get a consensus for all nodes in tNetwork and then
// verify that all nodes give the same reply when calling 'Arbiter' method.
func TestArbiter(t *testing.T) {
	// Verify that assumed functionality works.
	if ok := t.Run("SETUP 1", TestInitSession); !ok {
		t.Fatalf("Expected TestInitSession to work, it did not")
	}
	if ok := t.Run("SETUP 2", TestCollectVotes); !ok {
		t.Fatalf("Expected TestCollectVotes to work, it did not")
	}

	network.resetSessionMember()
	id := common.NewRandID(10)

	if err := network.initSession(id, addrs); err != nil {
		t.Fatal(err)
	}

	tries := 50
	consensus := false
	for i := 0; i < tries; i++ {
		_consensus, err := network.collectVotes(id, addrs)
		if err != nil {
			t.Fatal(err)
		}
		if !_consensus {
			if err := network.initSession(id, addrs); err != nil {
				t.Fatalf("could not re-init network: %v", err)
			}
			continue
		}

		consensus = true
		t.Logf("consensus after %v retries", i)
		break
	}

	if !consensus {
		t.Fatalf("didn't reach consensus after %v tries, aborting", tries)
	}

	var arbiterAddr *Addr
	i := -1
	for addr := range network.nodes {
		i++
		var err error
		arbiterResp := ArbiterClient(addr, &err).Arbiter()
		if err != nil {
			t.Fatalf("net err for '%v': %v", addr.ToStr(), err)
		}
		if s := arbiterResp.Status; s != StatusOK {
			t.Log(i)
			t.Fatalf("unexpected status for '%v': %v", addr.ToStr(), s.ToStr())
		}
		// Set first.
		if arbiterAddr == nil {
			arbiterAddr = &arbiterResp.Addr
			continue
		}
		if !arbiterAddr.Comp(arbiterResp.Addr) {
			t.Fatalf("unequal arbiter")
		}
	}
}

// TestCleanup releases network resources.
func TestCleanup(t *testing.T) {
	network.stop()
}
