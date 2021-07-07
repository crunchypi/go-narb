package rpc

import (
	"github.com/crunchypi/go-narb/apsa/common"
)

// Addr abbreviates arbiterutils.Addr.
type Addr = common.Addr

// ID abbreviates arbiterutils.ID.
type ID = common.ID

type ArbiterData = common.ArbiterData

// StatusCode abbreviates arbiterutils.StatusCode.
type StatusCode = common.StatusCode

const (
	StatusDefault               = common.StatusDefault
	StatusInvalidSessionMembers = common.StatusInvalidSessionMembers
	StatusInvalidSessionID      = common.StatusInvalidSessionID
	StatusInSession             = common.StatusInSession
	StatusNotInSession          = common.StatusNotInSession
	StatusFailedVoteCollection  = common.StatusFailedVoteCollection
	StatusFailedVoteConsensus   = common.StatusFailedVoteConsensus
	StatusSessionExpired        = common.StatusSessionExpired
	StatusArbiterExpired        = common.StatusArbiterExpired
	StatusOK                    = common.StatusOK
	StatusNotInit               = common.StatusNotInit
)

type sessionMember interface {
	InitSession(sessionID ID, voteOpt []Addr) StatusCode
	CollectVotes(sessionID ID, voteOpt []Addr) StatusCode
	Vote(sessionID ID) (Addr, ID, StatusCode)
	Arbiter() (ArbiterData, StatusCode)
}

// arbiterClient handles connections to remote ArbiterServer(s). It is private
// because each connection needs an address, and ArbiterClient func (further
// down) ensures that by exposing this type in exchange for an address.
type arbiterClient struct {
	remoteAddr Addr
	netErr     *error
}

// ArbiterClient returns an *arbiterClient which is set up to connect to
// remoteAddr. This simply forces all outgoing connections to have an address.
// Example:
//	var err error
// 	ArbiterClient(Addr{IP:"localhost", Port:"1234"}, *err).Ping()
//  if err != nil {...}
func ArbiterClient(remoteAddr Addr, err *error) *arbiterClient {
	if err == nil {
		var e error
		err = &e
	}
	return &arbiterClient{remoteAddr: remoteAddr, netErr: err}
}

type ArbiterServer struct {
	// No locking, that should be handled by ArbiterSessionMember.
	ArbiterSessionMember sessionMember
}

func NewArbiterServer(arbiterSessionState sessionMember) *ArbiterServer {
	return &ArbiterServer{arbiterSessionState}
}
