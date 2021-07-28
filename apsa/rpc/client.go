/*
RPC client for apsa. These methods implement the 'sessionMember' interface
found in ./rpc.go. Note, some of these methods use types as arguments while
connecting to the server counterpart, those are defined in ./server.go.

*/
package rpc

import "net/rpc"

// Ping calls the method with the same name on a remote ArbiterServer instance.
func (c *arbiterClient) Ping() bool {
	client, err := rpc.Dial("tcp", c.remoteAddr.ToStr())
	if err != nil {
		*c.netErr = err
		return false
	}
	defer client.Close()

	var resp bool
	*c.netErr = client.Call("ArbiterServer.Ping", 0, &resp)
	return resp
}

// InitSession calls the method with the same name on a remote ArbiterServer
// instance. See ./server.go and meaning of args & status code in
// ../sessionmember/sessionmember.go (file doc and method with the same name).
func (c *arbiterClient) InitSession(id ID, voteOpt []Addr) StatusCode {
	client, err := rpc.Dial("tcp", c.remoteAddr.ToStr())
	if err != nil {
		*c.netErr = err
		return StatusDefault
	}
	defer client.Close()
	args := InitSessionArgs{ID: id, VoteOpt: voteOpt}
	resp := StatusDefault
	*c.netErr = client.Call("ArbiterServer.InitSession", args, &resp)
	return resp
}

// Vote calls the method with the same name on a remote ArbiterServer
// instance. See ./server.go and meaning of args & status code in
// ../sessionmember/sessionmember.go (file doc and method with the same name).
func (c *arbiterClient) Vote(sessionID ID) (Addr, ID, StatusCode) {
	client, err := rpc.Dial("tcp", c.remoteAddr.ToStr())
	if err != nil {
		*c.netErr = err
		return Addr{}, ID(""), StatusDefault
	}
	defer client.Close()
	var resp VoteResp
	*c.netErr = client.Call("ArbiterServer.Vote", sessionID, &resp)
	return resp.Addr, resp.ID, resp.Status
}

// CollectVotes calls the method with the same name on a remote ArbiterServer
// instance. See ./server.go and meaning of args & status code in
// ../sessionmember/sessionmember.go (file doc and method with the same name).
func (c *arbiterClient) CollectVotes(sessionID ID, voteOpt []Addr) StatusCode {
	client, err := rpc.Dial("tcp", c.remoteAddr.ToStr())
	if err != nil {
		*c.netErr = err
		return StatusDefault
	}
	defer client.Close()
	arg := CollectVotesArg{SessionID: sessionID, VoteOpt: voteOpt}
	resp := StatusDefault
	*c.netErr = client.Call("ArbiterServer.CollectVotes", arg, &resp)
	return resp
}

// Arbiter calls the method with the same name on a remote ArbiterServer
// instance. See ./server.go and meaning of args & status code in
// ../sessionmember/sessionmember.go (file doc and method with the same name).
func (c *arbiterClient) Arbiter() ArbiterResp {
	client, err := rpc.Dial("tcp", c.remoteAddr.ToStr())
	if err != nil {
		*c.netErr = err
		return ArbiterResp{}
	}
	defer client.Close()
	var resp ArbiterResp
	*c.netErr = client.Call("ArbiterServer.Arbiter", 0, &resp)
	return resp

}
