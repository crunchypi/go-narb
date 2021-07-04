/*
This file contains status codes common for apsa, shared by
SessionMembers (arbiter session state) and the RPC impl.
*/
package common

import "fmt"

// StatusCode represents a status, much like http status codes.
type StatusCode int

const (
	// StatusDefault represents an unset/default StatusCode.
	StatusDefault StatusCode = iota
	// StatusInvalidSessionMembers represents a situation
	// where a arbiter vote session member (those who vote
	// for arbiters) gets vote options (slice of Addr,
	// defined in ./net.go) that is not whitelisted.
	StatusInvalidSessionMembers
	// StatusInvalidSessionID represents a scenario where
	// an incorrect/unknown ID is used in a session. An
	// example is when a vote session is restarted with
	// an ID that wasn't used in the previous session.
	StatusInvalidSessionID
	// StatusInSession represents a situation where it
	// is expected that an arbiter vote session is _not_
	// ongoing. This might occur if it is attempted to
	// start a session when one is already ongoing.
	StatusInSession
	// StatusNotInSession is the opposite of StatusInSession.
	// This might occur when it is attempted to collect arbiter
	// votes when a session is not initialised.
	StatusNotInSession
	// StatusFailedVoteCollection represents a state where
	// an arbiter vote session member failed to collect
	// arbiter votes from other session members.
	StatusFailedVoteCollection
	// StatusFailedVoteConsensus represents a state where
	// StatusFailedVoteCollection did not happen (i.e
	// votes were successfully collected), but a clear
	// consensus was not met.
	StatusFailedVoteConsensus
	// StatusSessionExpired represents a state where
	// an arbiter vote session expired.
	StatusSessionExpired
	// StatusArbiterExpired represents a state where
	// an arbiter exists but is expired.
	StatusArbiterExpired
	// StatusOK means that an operation succeeded.
	StatusOK
	// StatusNotInit is a debug state.
	StatusNotInit
)

// ToStr converts a StatusCode into a string for logging/debugging.
func (s *StatusCode) ToStr() string {
	switch *s {
	case 0:
		return "default / not set"
	case 1:
		return "used a list of session members but some are not whitelisted"
	case 2:
		return "got an invalid arbiter session id"
	case 3:
		return "in an arbiter vote session"
	case 4:
		return "not in arbiter vote session"
	case 5:
		return "failed collecting votes while in session"
	case 6:
		return "no vote consensus"
	case 7:
		return "arbiter session expired"
	case 8:
		return "arbiter expired"
	case 9:
		return "ok"
	case 10:
		return "uninitialised"
	}
	panic(fmt.Sprintf("option not implemented: %v", *s))
}
