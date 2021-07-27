/*
File contains methods of arbiterClients (plural), which does arbiterClient
(singular) operations for multiple addresses. Here, additional orchestration
operations are defined as well.

*/
package rpc

import (
	"github.com/crunchypi/go-narb/apsa/common"
)

// InitSession is equivalent to ArbiterClient(x).InitSession(...) for
// all x in remoteAddrs that were specified while setting up this client.
// Likewise, network errors are stored in the map specified during the client
// setup as well (status codes are not relevant here). The return is an Addr
// slice for remote nodes that were alive.
func (c *arbiterClients) Ping() []Addr {
	type pingResp struct {
		addr Addr
		ok   bool
		err  error
	}
	// Concurrent ping.
	ch := make(chan pingResp, len(c.remoteAddrs))
	for _, addr := range c.remoteAddrs {
		go func(addr Addr) {
			var err error
			ok := ArbiterClient(addr, &err).Ping()
			ch <- pingResp{addr: addr, ok: ok, err: err}
		}(addr)
	}

	// Collect reponses.
	responders := make([]Addr, 0, len(c.remoteAddrs))
	for i := 0; i < len(c.remoteAddrs); i++ {
		resp := <-ch
		c.errors[resp.addr] = resp.err
		if resp.ok {
			responders = append(responders, resp.addr)
		}
	}
	return responders
}

// InitSession is equivalent to ArbiterClient(x).InitSession(...) for
// all x in remoteAddrs that were specified while setting up this client.
// Likewise, network errors and status codes are stored in the maps specified
// during the client setup as well. The return is a bool indicating whether or
// not all nodes started (or restarted) a new arbiter vote session successfully.
func (c *arbiterClients) InitSession(id ID, voteOpt []Addr) bool {
	type initResp struct {
		addr   Addr
		status StatusCode
		err    error
	}

	// Conurrent init.
	ch := make(chan initResp, len(c.remoteAddrs))
	for _, addr := range c.remoteAddrs {
		go func(addr Addr) {
			var err error
			status := ArbiterClient(addr, &err).InitSession(id, voteOpt)
			ch <- initResp{addr: addr, status: status, err: err}
		}(addr)
	}

	// Collect responses.
	allOK := true
	for i := 0; i < len(c.remoteAddrs); i++ {
		resp := <-ch
		c.statuses[resp.addr] = resp.status
		c.errors[resp.addr] = resp.err
		if resp.status != StatusOK || resp.err != nil {
			allOK = false
		}
	}
	return allOK
}

// CollectVotes is equivalent to ArbiterClient(x).CollectVotes(...) for
// all x in remoteAddrs that were specified while setting up this client.
// Likewise, network errors and status codes are stored in the maps specified
// during the client setup as well. The returned bool will be true if all
// nodes could collect votes from eachother _and_ there was a consensus, the
// details of this can be checked in the aforementioned map containing status
// codes.
func (c *arbiterClients) CollectVotes(sessionID ID, voteOpt []Addr) bool {
	// Note: this is done in a blocking fashion on purpose, it's how the
	// arbiter consensus algorithm works. Each call to (client.go singular)
	// arbiterClient.CollectVotes will cause the remote arbiter server
	// to initiate (their) internal arbiterClient.Vote for each addr in
	// voteOpt (that, however, is done concurrently).
	consensus := true
	for _, addr := range c.remoteAddrs {
		var err error
		status := ArbiterClient(addr, &err).CollectVotes(sessionID, voteOpt)

		c.errors[addr] = err
		c.statuses[addr] = status
		if status != StatusOK {
			consensus = false
		}
	}
	return consensus
}

// Arbiter is equivalent to ArbiterClient(x).Arbiter(...) for for all x in
// remoteAddrs that were specified while setting up this client. Likewise,
// network errors and status codes are stored in the maps specified during the
// client setup as well. The return is a type containing arbiter data and a bool
// indicating whether or not all remote nodes aggreed on that arbiter (discard
// the arbiter data if there isn't a consensus). If there is an issue, check errors
// and status codes in the aforementioned maps.
func (c *arbiterClients) Arbiter() (ArbiterResp, bool) {
	type resp struct {
		addr Addr
		err  error
		resp ArbiterResp
	}
	// Concurrent remote calls.
	ch := make(chan resp, len(c.remoteAddrs))
	for _, addr := range c.remoteAddrs {
		go func(addr Addr) {
			var err error
			r := ArbiterClient(addr, &err).Arbiter()
			ch <- resp{addr: addr, err: err, resp: r}
		}(addr)
	}

	// Collect.
	consensus := true
	var arbiterTemp *ArbiterResp
	for i := 0; i < len(c.remoteAddrs); i++ {
		r := <-ch

		c.statuses[r.addr] = r.resp.Status
		c.errors[r.addr] = r.err

		// Setting on first iter.
		if arbiterTemp == nil {
			arbiterTemp = &r.resp
			continue
		}
		if !arbiterTemp.Addr.Comp(r.resp.Addr) {
			consensus = false
		}
	}

	var arbiterResp ArbiterResp
	if arbiterTemp != nil {
		arbiterResp = *arbiterTemp
	}
	return arbiterResp, consensus
}

func (c *arbiterClients) uniformStatus(status StatusCode) bool {
	for _, s := range c.statuses {
		if s != status {
			return false
		}
	}
	return true
}

// TryForceNewArbiter is a helper that tries to automate the arbiter selection
// process. It does the following:
// - arbiterClients.Ping() and filter out non-responding nodes.
// - arbiterClients.InitSession(...) and abort on fail.
// - arbiterClients.CollectVotes(...) until consensus is reached or 'retries'
//   has been exceeded. Note, if a retry is attempted and at least one node
//   responds with a status code other than StatusFailedVoteConsensus, then
//   this routine is aborted entirely, as the network state could be complicated.
//   In this scenario, the caller has to check the status codes & errors maps
//   specified while setting up this client.
func (c *arbiterClients) TryForceNewArbiter(retries int) bool {
	available := c.Ping()
	id := common.NewRandID(20)
	ok := c.InitSession(id, available)
	if !ok {
		return false
	}
	consensus := false
	for i := 0; i < retries; i++ {
		ok = c.CollectVotes(id, available)
		if !ok {
			// Simply no consensus, retry.
			if c.uniformStatus(StatusFailedVoteConsensus) {
				c.InitSession(id, available)
				continue
			}
			// Some other case, such as StatusFailedVoteCollection,
			// might have been caused by something this func can't
			// predict, so return with current c.errors and statuses.
			break
		}
		consensus = true
		break
	}
	return consensus
}
