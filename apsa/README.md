# APSA network arbitration algorithm

APSA is an approach to network consensus, where an arbiter is selected out of a set of nodes for the purpose of deciding an action.
It is short for Async, Partial-Sync, Async -- primarily a description of how the 3 steps in the algorithm are meant to be performed.
This repo segment contains an impelemtation for APSA in Go, with an RPC layer on top.
<br>

### Index:
- [Algorithm description](#algorithm-description)
- [States and status codes](#states-and-status-codes)
- [Code example](#examples)
<br>

# Algorithm description
There are just three steps but they have to be performed in a specific way to prevent deadlocks. There are also a lot of state,
that is described [further down](#states-and-status-codes).

Simplified overview of steps:
- (1): Init an arbiter voting session.
- (2): Make all session members (i.e nodes) vote and collect votes.
- (2): Give arbiter description.

Detailed description with pseudocode:
let S = set of all nodes in a network that follow this protocol.
let N = any node in S.

let InitSession = func that initiates a session.
let CollectVotes = func that makes a node collect votes from all other nodes.
let Vote = func that gives out an arbiter vote (called by remote nodes).
let Arbiter = func that gives out an arbiter.

##### Step 1
Being N, call InitSession on all nodes in S. The func accepts two args; (A) an ID associated for the corrent session and
(B) a list of addresses that can be used to reach all nodes that are a part of this new voting session. If B is not a subset of
N (i.e whitelisted), then this process should be rejected, though the primary purpose of B is for the next step. The purpose of
arg A is also primarily related to the next step; it is stored in all nodes in the B list and prevents session overlaps (so
new sessions can't normally be started or used without this ID). Also, the InitSession func/method should also create a new
local vote (out of B) for all nodes. The next action to take, if the process is rejected on any node, will depend on the internal
state, this will be explained more in the [States](#states-and-status-codes) section.
  
NOTE: N can do the InitSession call on all nodes in S concurrently.

##### Step 2
Being N, call CollectVotes for all nodes in S. This func accepts the same arguments as InitSession used in step 1. Again,
if the address list is not recognised, then the process should fail. If the ID doesn't match the ID stored from the InitSession
call, then the process should also fail. If these checks are ok, then the node that gets this CollectVotes call should collect
votes by calling Vote on all nodes that are in the address list set up in step 1, except for itself. This collection process
can have two outcomes; (A) one of the nodes in the aforementioned address list is unreachable or (B) all nodes votes but there
was no consensus. If A, then the global voting process failed and should be restarted with a new address list (that does not
include unreachable nodes), if B, then this step (no. 2) should be repeated. In either case, N should not abort calling CollectVotes
on all nodes in S, as that will lead to an inconsistency in the network. For more info in the [States](#states-and-status-codes) section.
  
NOTE: N must do the CollectVotes call on all nodes in S in a _blocking_ way, but the nodes that are being called should do
the vote collection (i.e other.Vote call on all other nodes) concurrently. So a partial sync, in a way.
  
##### Step 3
If step 2 is done successfully, then calling Arbiter on any nodes should give the same response. It might not be the case
if the nodes in S are not chronologically synced and a voting session result gets expired somewhere, so a majority vote can be
done with this step.
  
NOTE: Calling Arbiter for a majority consensus (not to be confused with voting consensus in step 2) can also be done concurrently.
  

# States and status codes
Here is some description of states and status codes related to the actions/funcs described in the [Algorithm description](#algorithm-description)
section:


#### InitSession
- StatusInvalidSessionMembers: The vote options that are given for this func/method/step contains addresses that are not whitelisted.
- StatusInSession: A session is already ongoing and not failed.
- StatusInvalidSessionID: A session can be restarted _only_ if it failed. The fail state is set with the CollectVotes step ([step 2](#step-2))
  and can be caused by either votes that led to no vote consensus, or if a node failed to collect a vote (i.e if a voter is offline). With this status code,
  a voting session is expected to be restarted with InitSession, but that will only work with the same session ID as last time. If that ID is not the same,
  then this status code is returned.
- StatusOK: Address list and ID is stored, a local vote is set, and [step 2](#step-2) is awaited.

#### CollectVotes
- StatusInvalidSessionMembers: The vote options that are given for this func/method/step contains addresses that are not whitelisted.
- StatusNotInSession: [step 1, InitSession](#step-1) was not called and a voting round is not expected.
- StatusInvalidSessionID: A session ID was set in [step 1, call to InitSession](#step-1) but it is not the same as the one
  given for the CollectVotes step/func/method.
- StatusFailedVoteCollection: The voting round was attempted for the node that has this func/method called on, but not all voters/nodes
  that are in the address options list responded normally/successfully. The reason could be that some voters were unreachable, or some
  other factors (see [status codes for Vote](#vote)). If this status code is returned then the vote session should continue for all other nodes
  (so network is in a consistent state) and then be restarted.
- StatusFailedVoteConsensus: A node that gets the CollectVotes call will attempt to call all other nodes in the address list, but there might
  not be a clear consensus. In this case, the vote session should continue for all nodes (so network is in a consistent state) and then be restarted.
  
#### Vote
- StatusInvalidSessionID: A session ID was set in [step 1, call to InitSession](#step-1) but it is not the same as the one
  given for the Vote step/func/method.
- StatusSessionExpired: [step 1, a call to InitSession](#step-1) was done but the session expired.
- StatusNotInSession: A session was not started with a call to [InitSession (step 1)](#step-1).
- StatusOK: No issue, a valid vote is returned.
- StatusFailedVoteCollection: Same meaning as the status with the same name given by [CollectVotes](#collectvotes).
- StatusFailedVoteConsisnsus: Same meaning as the status with the same name given by [CollectVotes](#collectvotes).

#### Arbiter
- StatusInSession: A voting session was started but not completed [step 1 succeeded but step 2 was not completed](#step-1).
- StatusFailedVoteCollection: Same meaning as the status with the same name given by [CollectVotes](#collectvotes).
- StatusFailedVoteConsensus: Same meaning as the status with the same name given by [CollectVotes](#collectvotes).
- StatusArbiterExpired: Arbiter was set from the previous voting session but expired.
- StatusOK: Arbiter exists and returned.


# Examples

(All of this code is found in apsa/examples/examples_test.go)

The code in this repo implements an RPC layer (apsa/rpc) on top of the APSA algorithm (apsa/sessionmember) and these examples will focus on
the RPC stuff. There are two client types (1) arbiterClient and (2) arbiterClients (note plural). The former simply interfaces remote sessionmember
instances, so the interface is the same. The latter, on the other hand, is an abstraction and orchestration of the former. For instance,
the algorithm expectes that InitSession is called on all nodes and that requires 20-30 lines of code if one uses goroutines.
The abstraction/orchestration type (arbiterClients plural) does that conveniently with one call.

<br>
The examples will cover apsa/rpc.arbiterClients (plural) for convenience. If there are edge cases and more control is needed,
see how rpc.arbiterClient (singular) is used from the plural alternative. That can be found here: https://github.com/crunchypi/go-narb/blob/docs/apsa/rpc/clients.go.

<br>

Before listing examples, this code will be common (it's put here to reduce example size)
```go
import (
	"testing"
	"time"

	"github.com/crunchypi/go-narb/apsa/common"
	"github.com/crunchypi/go-narb/apsa/rpc"
	"github.com/crunchypi/go-narb/apsa/sessionmember"
  
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
)
```

Ping all nodes and get a slice of addresses of repliers:
```go

...the-setup-code-listed-further-up....


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
```

Call InitSession on multiple remote nodes:
```go

...the-setup-code-listed-further-up....

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

```

Call CollectVotes on multiple remote nodes:
```go

...the-setup-code-listed-further-up....

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
```

Call Arbiter on multiple remote nodes:
```go

...the-setup-code-listed-further-up....

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

```


Convenience method: TryForceNewArbiter. It does some of the stuff listed in the previous examples
(calling Ping, InitSession and CollectVotes) with one line. It also does retries automatically if
there is not consensus, which is nice.
```go

...the-setup-code-listed-further-up....

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
```

