# APSA network arbitration algorithm

APSA is an approach to network consensus, where an arbiter is selected out of a set of nodes for the purpose of deciding an action.
It is short for Async, Partial-Sync, Async -- primarily a description of how the 3 steps in the algorithm are meant to be performed.
This repo segment contains an impelemtation for APSA in Go, with an RPC layer on top.
<br>

### Index:
- [Algorithm description](#algorithm-description)
- [States and status codes](#states-and-status-codes)
- [Code example](examples)
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


##### InitSession
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

These are written for the arbiterClient type in apsa/rpc, it communicates with ArbiterServer (type in the same pkg), which is just an RPC layer
on top of apsa/sessionmember.

Call InitSession (step 1)[#step-1] on some nodes.
```


```




