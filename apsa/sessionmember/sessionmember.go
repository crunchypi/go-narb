/*
Implementation of the apsa algorithm.
Descr: https://github.com/crunchypi/go-narb/tree/docs/apsa#algorithm-description

*/
package sessionmember

import (
	"sync"
	"time"

	"github.com/crunchypi/go-narb/apsa/common"
)

type sessionState int

const (
	// SessionMember instantiated but never used.
	ready sessionState = iota
	// SessionMember.InitSession called.
	session
	// last state was session and SessionMember.CollectVotes was called but
	// failed either because there was no clear arbiter consensus, or at least
	// one remote node failed to vote.
	failed
	// last state was session and SessionMember.CollectVotes was called and
	// succeeded: an arbiter was found.
	done
)

const (
	// Abbreviation of status of same name in common pkg.
	StatusDefault = common.StatusDefault
	// Abbreviation of status of same name in common pkg.
	StatusInvalidSessionMembers = common.StatusInvalidSessionMembers
	// Abbreviation of status of same name in common pkg.
	StatusInvalidSessionID = common.StatusInvalidSessionID
	// Abbreviation of status of same name in common pkg.
	StatusInSession = common.StatusInSession
	// Abbreviation of status of same name in common pkg.
	StatusNotInSession = common.StatusNotInSession
	// Abbreviation of status of same name in common pkg.
	StatusFailedVoteCollection = common.StatusFailedVoteCollection
	// Abbreviation of status of same name in common pkg.
	StatusFailedVoteConsensus = common.StatusFailedVoteConsensus
	// Abbreviation of status of same name in common pkg.
	StatusSessionExpired = common.StatusSessionExpired
	// Abbreviation of status of same name in common pkg.
	StatusArbiterExpired = common.StatusArbiterExpired
	// Abbreviation of status of same name in common pkg.
	StatusOK = common.StatusOK
	// Abbreviation of status of same name in common pkg.
	StatusNotInit = common.StatusNotInit
)

// SessionMember holds the exported API for this pkg, along with relevant
// state. See documentation at the top of this file for more info.
type SessionMember struct {
	state     sessionState
	localAddr Addr
	// Sessions can't be started with addresses not in here.
	whitelist []Addr

	// Session data for each session (such as vote table and local vote).
	sessionData sessionData
	// How long the next session can exist, should be relatively short,
	// at least short enough for all nodes to contact eachother.
	nextSessionDuration time.Duration

	// Chosen arbiter from last successful voting round.
	arbiterData ArbiterData
	// How long an arbiter (arbiterData field) can be valid.
	nextArbiterDuration time.Duration

	// Funcs that separate networking from this pkg. At the time of
	// writing, it only consists of one func which is suppused to
	// call a _remote_ SessionMember.Vote. See ./data.go definition.
	f Funcs

	// Async safe as long as SessionMember instances follow the rules
	// specified at the top of this file (async, partial sync, async).
	sync.Mutex
}

// NewSessionMemberConfig is used as an argument to the NewSessionMember func.
type NewSessionMemberConfig struct {
	// LocalAddr specifies the local network address for a SessionMember.
	LocalAddr Addr
	// Whitelist is used to reject any vote options when calling
	// SessionMember.InitSession and SessionMember.CollectVotes.
	Whitelist []Addr
	// F is a collection of functionality that is required by SessionMember
	// but with a separated implementation (see ./data.go definition).
	F Funcs
	// SessionDuration specifies how long a session can last. Should be short
	// but long enough for all SessionMember network nodes to call eachother.
	SessionDuration time.Duration
	// ArbiterDuration specifies how long an arbiter can last.
	ArbiterDuration time.Duration
}

// NewSessionMember uses a configuration to create a new SessionMember.
func NewSessionMember(cfg NewSessionMemberConfig) *SessionMember {
	if !cfg.F.ok() {
		panic("F field of cfg invalid, contains nil func(s)")
	}
	return &SessionMember{
		localAddr:           cfg.LocalAddr,
		whitelist:           cfg.Whitelist,
		nextSessionDuration: cfg.SessionDuration,
		nextArbiterDuration: cfg.ArbiterDuration,
		f:                   cfg.F,
	}
}

// Helper method to check if a slice of Addr are in the internal whitelist.
func (s *SessionMember) inWhitelist(a []Addr) bool {
	for _, item := range a {
		if !item.In(s.whitelist) {
			return false
		}
	}
	return true
}

// InitSession attempts to start a new session or restart an already exsisting
// session if the previous one failed (in that case the sessionID must be the
// same). Can return a few StatusCode variants for different scenarios:
//
//	StatusInvalidSessionMembers
//		The second arg (vote options) contains addresses that are not
//		whitelisted.
//
//	StatusInSession
//		A session is already ongoing and not failed.
//
//	StatusInvalidSessionID
//		A session can be restarted _only_ if it failed. Fail cases are set with
//		calls to SessionMember.CollectVotes and can either be 'no vote consensus'
//		or 'failed vote collection'. In that case, a session is expected to be
//		restarted, but only with the same session ID. if that ID is not the same
//		then this status code is returned,
//
//	StatusOK
//		Session started successfully.
//
func (s *SessionMember) InitSession(sessionID ID, voteOpt []Addr) StatusCode {
	s.Lock()
	defer s.Unlock()

	// All proposed vote options must be in the whitelist.
	if !s.inWhitelist(voteOpt) {
		return StatusInvalidSessionMembers
	}
	// Session can only be started with states: ready, failed.
	if s.state == session { //s.state != ready && s.state != failed {
		return StatusInSession
	}
	// Retrying a session can only be done with an already known id.
	if s.state == failed && sessionID != s.sessionData.sessionID {
		return StatusInvalidSessionID
	}

	s.sessionData = newSessionData(newSessionDataArgs{
		sessionID:       sessionID,
		voteOpt:         voteOpt,
		sessionDuration: s.nextSessionDuration,
	})
	s.state = session
	return StatusOK
}

// collectVotes is meant to be used by SessionMember.CollectVotes (capital).
// it collects votes from addresses in voteOpt (1 goroutine for each).
// potential votes are added to internal voting table.
func (s *SessionMember) collectVotes(sessionID ID, voteOpt []Addr) {
	voteOpt = s.localAddr.FilterFrom(voteOpt)
	type resp struct {
		remoteVoter  Addr
		remoteVote   Addr
		remoteID     ID
		remoteStatus StatusCode
	}
	// Async contact remote.
	ch := make(chan resp, len(voteOpt))
	for _, addr := range voteOpt {
		go func(addr Addr) {
			vote, id, status := s.f.RemoteVoteFunc(addr, sessionID)
			ch <- resp{addr, vote, id, status}
		}(addr)
	}

	// Collect.
	for i := 0; i < len(voteOpt); i++ {
		r := <-ch
		// This check is redundant, as validation can also done with
		// s.voteData.voteCompleted and s.voteData.voteConsensus, which
		// is done in s.CollectVotes (at the moment of writing). Still,
		// the chech is here just in case.
		if r.remoteStatus != StatusOK {
			continue
		}
		s.sessionData.addVote(r.remoteVoter, r.remoteVote, r.remoteID)
	}
}

// CollectVotes makes the local instance of SessionMember to collect votes from
// remote instances of SessionMember for each addr in voteOpt. Can return a few
// StatusCode variants for different scenarios:
//
//	StatusInvalidSessionMembers
//		The second arg (vote options) contains addresses that are not
//		whitelisted.
//
//	StatusNotInSession
//		Means that the SessionMember.InitSession was not called for this
//		instance, i.e a voting round is not expected.
//
//	StatusInvalidSessionID
//		Not the same sessionID arg as when the current vote session was
//		started with SessionMember.InitSession
//
//	StatusFailedVoteCollection
//		The voting round was attempted in the current instance but not all
//		voters/nodes responded successfully. This would depend on whether the
//		failed voters were online or not, or some other factors
//		(see SessionMember.Vote). If this is returned then the vote session
//		should continue for all other nodes (so network is in a consistent
//		state) and then restarted.
//
//	StatusFailedVoteConsensus
//		Means that all voters voted successfully but there isn't a clear
//		majority (for instance if there are two nodes and they both voted for
//		eachother). In this case, the vote session should continue for the rest
//		of the nodes (so network is in a consistent state) and then restarted.
//
// NOTE: Don't call this concurrently for all nodes.
//
func (s *SessionMember) CollectVotes(sessionID ID, voteOpt []Addr) StatusCode {
	s.Lock()
	defer s.Unlock()

	// All proposed vote options must be in the whitelist.
	if !s.inWhitelist(voteOpt) {
		return StatusInvalidSessionMembers
	}
	// This call can only be done while in a vote session.
	if s.state != session {
		return StatusNotInSession
	}
	// Vote collector must be the same entity as the session initiator.
	if sessionID != s.sessionData.sessionID {
		return StatusInvalidSessionID
	}

	sData := s.sessionData // Abbreviation.
	// Add local vote.
	sData.addVote(s.localAddr, sData.localVote, sData.localID)

	// Collect remote votes (don't contact self).
	voteOpt = s.localAddr.FilterFrom(voteOpt)
	s.collectVotes(sessionID, voteOpt)

	// False if some voters from the block above (s.f.remoteVoteFunc)
	// didn't succeed, or if voteOpt is not the entire set of keys in
	// s.voteData.table.
	if !sData.voteCompleted() {
		s.state = failed
		return StatusFailedVoteCollection
	}
	if !sData.voteConsensus() {
		s.state = failed
		return StatusFailedVoteConsensus
	}

	s.state = done
	arbiter, _ := sData.nextArbiter(s.nextArbiterDuration)
	s.arbiterData = arbiter
	return StatusOK
}

// Vote is what SessionMember.CollectVotes method should call when collecting
// votes from remote nodes, so this is not intended to be used on a local
// instance. Can return different status codes for different scenarios:
//
//	StatusInvalidSessionID
//		The local instance of SessionMember got an ID when InitSession was
//		called, but it is not the same ID as the one gotten as an argument
//		here.
//
//	StatusSessionExpired
//		A local session was started with InitSession and the correct ID was
//		given for this method call, but the session has expired.
//
//	StatusNotInSession
//		A session was not started locally.
//
//	StatusOK
//		No issue, a valid vote is returned.
//
//	StatusFailedVoteCollection.
//		SessionMember.CollectVotes was called on this local node before
//		this method was invoked, and the vote collection round _failed_
//		because at least one remtote sessionmember didn't respond with
//		StatusOK when this method was called on it.
//
//	StatusFailedVoteConsensus
//		SessionMember.CollectVotes was called on this local node before
//		this method was invoked, and the vote collection _succeeded_, but
//		no consensus was found.
//
func (s *SessionMember) Vote(sessionID ID) (Addr, ID, StatusCode) {
	s.Lock()
	defer s.Unlock()

	sData := s.sessionData // Abbreviation.

	// Convenience: no data + status code.
	emptyWith := func(status StatusCode) (Addr, ID, StatusCode) {
		return Addr{}, ID(""), status
	}
	// Convenience: config data with variable status.
	dataWith := func(status StatusCode) (Addr, ID, StatusCode) {
		return sData.localVote, sData.localID, status
	}

	if sessionID != sData.sessionID {
		return emptyWith(StatusInvalidSessionID)
	}
	if sData.expired() {
		return emptyWith(StatusSessionExpired)
	}

	switch s.state {
	case ready:
		return emptyWith(StatusNotInSession)

	case session:
		return dataWith(StatusOK)

	case failed:
		if !sData.voteCompleted() {
			return dataWith(StatusFailedVoteCollection)
		}
		if !sData.voteConsensus() {
			return dataWith(StatusFailedVoteConsensus)
		}

	case done:
		return dataWith(StatusOK)
	}

	// Unhandled.
	return emptyWith(StatusDefault)
}

// Arbiter attempts to give the arbiter, a result of the last successful
// arbiter vote session. Can return a few status codes:
//
//	StatusNotInit
//		Special case when a sessionmember is instantiated in code but the
//		InitSession method was never called.
//
//	StatusInSession
//		Local SessionMember.InitSession was called but not CollectVotes,
//		so a voting session is ongoing and calling this method is premature.
//
//	StatusFailedVoteCollection
//		The voting round was attempted in the current instance but not all
//		voters/nodes responded successfully. This would depend on whether the
//		failed voters were online or not, or some other factors
//		(see SessionMember.Vote). If this is returned then the vote session
//		should continue for all other nodes (so network is in a consistent
//		state) and then restarted.
//
//	StatusFailedVoteConsensus
//		Means that all voters voted successfully but there isn't a clear
//		majority (for instance if there are two nodes and they both voted for
//		eachother). In this case, the vote session should continue for the rest
//		of the nodes (so network is in a consistent state) and then restarted.
//
//	StatusArbiterExpired
//		An arbiter exists but it expired.
//
//	StatusOK
//		Arbiter exists and returned with this status code.
//
//	StatusDefault
//		Edge case that was not accounted for (bug).
//
func (s *SessionMember) Arbiter() (ArbiterData, StatusCode) {
	s.Lock()
	defer s.Unlock()

	emptyWith := func(status StatusCode) (ArbiterData, StatusCode) {
		return ArbiterData{}, status
	}

	switch s.state {
	case ready:
		return emptyWith(StatusNotInit)

	case session:
		return emptyWith(StatusInSession)

	case failed:
		if !s.sessionData.voteCompleted() {
			return emptyWith(StatusFailedVoteCollection)
		}
		if !s.sessionData.voteConsensus() {
			return emptyWith(StatusFailedVoteConsensus)
		}

	case done:
		if s.arbiterData.Expired() {
			return emptyWith(StatusArbiterExpired)
		}
		return s.arbiterData, StatusOK
	}

	// Unhandled.
	return emptyWith(StatusDefault)
}
