/*
See apsa/README.md for more details:

The code in this repo implements an RPC layer (apsa/rpc) on top of the APSA
algorithm (apsa/sessionmember) and these examples will focus on the RPC stuff.
There are two client types; (1) arbiterClient and (2) arbiterClients (plural).
The former simply interfaces remote SessionMember instances instances, so the
interface is the same, while the latter is an abstraction/orchestration of the
former.

For instance, the algorithm expects that InitSession is called on all nodes
and that requires 20-30 lines of code if one were to use rpc.arbiterClient
with goroutines. The abstraction/orchestration type (arbiterClients, plural)
does that conveniently with one call.


*/
package examples

import (
	"testing"
	"time"

	"github.com/crunchypi/go-narb/apsa/common"
	"github.com/crunchypi/go-narb/apsa/rpc"
	"github.com/crunchypi/go-narb/apsa/sessionmember"
)

/*
--------------------------------------------------------------------------------
This type holds RPC server instances which the examples further down
will connect to. It also contins a map 'stopFuncs' for funcs that
will shut-down each server.
--------------------------------------------------------------------------------
*/

type network struct {
	nodes     map[common.Addr]*rpc.ArbiterServer
	stopFuncs map[common.Addr]func()
}

/*
--------------------------------------------------------------------------------
Factory func for the 'network' type. It does the following for all addrs:
- Create a rpc/sessionmember.SessionMember
- Use that SessionMember to create a server.
- Start server.
- Put server and stop func into the 'network' type.
--------------------------------------------------------------------------------
*/

func newNetwork(addrs []common.Addr) *network {
	r := network{
		nodes:     make(map[common.Addr]*rpc.ArbiterServer, len(addrs)),
		stopFuncs: make(map[common.Addr]func(), len(addrs)),
	}
	for _, addr := range addrs {
		// A sessionmember should be able to call x.Vote() on a remove
		// node. The 'sessionmember' pkg has no knowledge of actual network
		// calls, so that has to be given from somewhere else -- in this
		// case 'rpc' with RemoteVoteFunc().
		f := sessionmember.Funcs{RemoteVoteFunc: rpc.RemoteVoteFunc()}
		cfg := sessionmember.NewSessionMemberConfig{
			LocalAddr:       addr,
			Whitelist:       addrs,
			F:               f,
			SessionDuration: time.Second * 5,
			ArbiterDuration: time.Minute * 5,
		}

		// This implements the actual apsa algorithm.
		sessionMember := sessionmember.NewSessionMember(cfg)
		// RPC server on top of sessionmember.SessionMember
		server := rpc.NewArbiterServer(sessionMember)
		// Func to call for stopping the server.
		stopServerFunc, err := rpc.StartListen(server, addr)
		if err != nil {
			// ...
		}
		r.nodes[addr] = server
		r.stopFuncs[addr] = stopServerFunc

	}
	return &r
}

func (n *network) cleanup() {
	for _, stopFunc := range n.stopFuncs {
		stopFunc()
	}
}

/*
--------------------------------------------------------------------------------
Examples below.


They will use rpc.arbiterClients (plural) for convenience. If there are edge
cases are some additional control is needed, then see apsa/rpc/clients.go for
demonstrations of how rpc.arbiterClient (singular) can be used... or the test
file in that dir.
--------------------------------------------------------------------------------
*/

// Example and not an actual test.
func TestClientsPing(t *testing.T) {
	addrs := []common.Addr{
		{"localhost", "3000"},
		{"localhost", "3001"},
		{"localhost", "3002"},
	}

	// Start servers.
	n := newNetwork(addrs)
	defer n.cleanup()

	errors := make(map[common.Addr]error)
	// Slice of responders is returned. Note, third arg isn't needed
	// -- it is a map[common.Addr]common.StatusCode, but the ping
	// doesn't use any status codes related to APSA.
	responders := rpc.ArbiterClients(addrs, errors, nil).Ping()

	// check errors.

	t.Log(responders)
}

// Example and not an actual test.
func TestClientsInitSession(t *testing.T) {
	addrs := []common.Addr{
		{"localhost", "3000"},
		{"localhost", "3001"},
		{"localhost", "3002"},
	}

	sessID := common.NewRandID(8)

	// Start servers.
	n := newNetwork(addrs)
	defer n.cleanup()

	errors := make(map[common.Addr]error)
	statuses := make(map[common.Addr]common.StatusCode)
	allInit := rpc.ArbiterClients(addrs, errors, statuses).InitSession(sessID, addrs)

	// check errors.

	t.Log(allInit)
}

// Example and not an actual test.
func TestClientsCollectVotes(t *testing.T) {
	addrs := []common.Addr{
		{"localhost", "3000"},
		{"localhost", "3001"},
		{"localhost", "3002"},
	}

	sessID := common.NewRandID(8)

	// Start servers.
	n := newNetwork(addrs)
	defer n.cleanup()

	// apsa/README.md describes how InitSession has to be called before
	// CollectVotes, this line of code does that. Also note that errors
	// and status codes are ignored here for brevity.
	allInit := rpc.ArbiterClients(addrs, nil, nil).InitSession(sessID, addrs)
	if !allInit {
		t.Fatalf("whoups")
	}

	// Step 2, call CollectVotes for all addresses. See apsa/README.md.
	errors := make(map[common.Addr]error)
	statuses := make(map[common.Addr]common.StatusCode)
	consensus := rpc.ArbiterClients(addrs, errors, statuses).CollectVotes(sessID, addrs)

	// Might be false if no consensus or some other issue. Again, see apsa/README.md.
	t.Log(consensus)

}

// Example and not an actual test.
func TestClientsArbiter(t *testing.T) {
	addrs := []common.Addr{
		{"localhost", "3000"},
		{"localhost", "3001"},
		{"localhost", "3002"},
	}

	sessID := common.NewRandID(8)

	// Start servers.
	n := newNetwork(addrs)
	defer n.cleanup()

	// Doing step 1 & 2 in the APSA algorithm (see apsa/README.md). Errors
	// and status codes are ignored for brevity, see TestClientsInitSession
	// and TestClientsCollectVotes for better usage.
	rpc.ArbiterClients(addrs, nil, nil).InitSession(sessID, addrs)
	rpc.ArbiterClients(addrs, nil, nil).CollectVotes(sessID, addrs)

	// The previous line of code (using CollectVotes) returns a bool, if
	// it is failse, then there is no consensus, and doing the next lines
	// is pointless.

	errors := make(map[common.Addr]error)
	statuses := make(map[common.Addr]common.StatusCode)
	arbiter, ok := rpc.ArbiterClients(addrs, errors, statuses).Arbiter()

	// ... err and status checking ...

	if !ok {
		// ....
	}

	// Will only give an addr if consensus reach with step 2.
	t.Log(arbiter.Addr.IP != "")
}

// Example and not an actual test.
func TestClientsTryForceNewArbiter(t *testing.T) {
	addrs := []common.Addr{
		{"localhost", "3000"},
		{"localhost", "3001"},
		{"localhost", "3002"},
	}

	// Start servers.
	n := newNetwork(addrs)
	defer n.cleanup()

	errors := make(map[common.Addr]error)
	statuses := make(map[common.Addr]common.StatusCode)
	retries := 10
	// This automates calls InitSession and CollectVotes for all nodes (also
	// automatically filters out dead nodes using the Ping method.). 10 might
	// be overkill for retries but doesn't hurt.
	ok := rpc.ArbiterClients(addrs, errors, statuses).TryForceNewArbiter(retries)

	// ... err and status checking ...

	if !ok {
		// ...
	}

	arbiter, ok := rpc.ArbiterClients(addrs, errors, statuses).Arbiter()

	// ... err and status checking ...

	if !ok {
		// ...
	}

	// APSA isn't deterministic but TryForceNewArbiter with a high retries args
	// should force a consensus.
	t.Log(arbiter.Addr.IP != "")
}
