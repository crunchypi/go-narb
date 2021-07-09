/*
RPC server for apsa. All of these methods simply forward calls and responses
to/from the internal apsa sessiomember (methods with the same name). More docs
for meaning of arguments and replies is found in
../sessionmember/sessionmember.go
*/
package rpc

import (
	"errors"
	"time"
)

// checkNil checks for nil internal data and second RPC arg.
func checkNil(s sessionMember, resp interface{}) error {
	if s == nil {
		return errors.New("arbiter session member not set")
	}
	if resp == nil {
		return errors.New("nil response arg for RPC func.")
	}
	return nil
}

// Ping is an endpoint used for checking if a node is alive.
func (s *ArbiterServer) Ping(_ int, resp *bool) error {
	*resp = true
	return nil
}

// InitSessionArgs is used for ArbiterServer.InitSession.
type InitSessionArgs struct {
	ID      ID
	VoteOpt []Addr
}

// InitSession forwards the call and resp to/from the internal ArbiterSessionmember.
func (s *ArbiterServer) InitSession(args InitSessionArgs, resp *StatusCode) error {
	if err := checkNil(s.ArbiterSessionMember, resp); err != nil {
		return err
	}
	*resp = s.ArbiterSessionMember.InitSession(args.ID, args.VoteOpt)
	return nil
}

// VoteResp is used as the reponse in ArbiterServer.Vote.
type VoteResp struct {
	Addr   Addr
	ID     ID
	Status StatusCode
}

// Vote forwards the call and resp to/from the internal ArbiterSessionMember.
func (s *ArbiterServer) Vote(sessionID ID, resp *VoteResp) error {
	if err := checkNil(s.ArbiterSessionMember, resp); err != nil {
		return err
	}

	addr, id, status := s.ArbiterSessionMember.Vote(sessionID)
	resp.Addr = addr
	resp.ID = id
	resp.Status = status
	return nil
}

// CollectVotesArg is used as the argument to ArbiterServer.CollectVotes.
type CollectVotesArg struct {
	SessionID ID
	VoteOpt   []Addr
}

// CollectVotes forward the call and resp to/from the internal ArbiterSessionMember.
func (s *ArbiterServer) CollectVotes(arg CollectVotesArg, resp *StatusCode) error {
	if err := checkNil(s.ArbiterSessionMember, resp); err != nil {
		return err
	}

	*resp = s.ArbiterSessionMember.CollectVotes(arg.SessionID, arg.VoteOpt)
	return nil
}

// ArbiterResp is the response used in ArbiterServer.Arbiter.
type ArbiterResp struct {
	Addr       Addr
	Status     StatusCode
	Expiration time.Time
}

// Arbiter forwards the call and resp to/from the internal ArbiterSessionMember.
func (s *ArbiterServer) Arbiter(_ int, resp *ArbiterResp) error {
	if err := checkNil(s.ArbiterSessionMember, resp); err != nil {
		return err
	}
	arbiter, status := s.ArbiterSessionMember.Arbiter()
	(*resp).Addr = arbiter.Addr
	(*resp).Status = status
	(*resp).Expiration = arbiter.Expiration
	return nil
}
