/*
Package contains an RPC implementation for apsa sessionmembers.

type ArbiterServer simply forwards calls and responses to/from
the internal sessionmember instance (methods with the same name),
while the ArbiterClient type calls the server (method names are
also mirrored).

*/
package rpc

import (
	"net"
	"net/rpc"

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

// arbiterClients handles 'arbiterClient' operations for multiple addresses,
// and has methods with the same name, though with a different usage. For
// instance, InitSession() stores all status codes internally (statuses field)
// and returns a bool instead, indicating whether or not _all_ nodes started
// a new vote session. This is useful because one would use 'arbiterClient'
// for multiple remote servers in this way anyway.
type arbiterClients struct {
	remoteAddrs []Addr
	statuses    map[Addr]StatusCode
	errors      map[Addr]error
}

// ArbiterClients returns an *arbiterClients instance which is set up to
// connect to all addrs in remoteAddrs. This simply forces all outgoing
// connections to have an address. Additionally, it accepts two map pointers,
// one for errors associated with an address, and another for status codes.
// Use nil for those if they are not of interest.
func ArbiterClients(remoteAddrs []Addr, errors map[Addr]error,
	statuses map[Addr]StatusCode) *arbiterClients {
	if errors == nil {
		errors = make(map[Addr]error)
	}
	if statuses == nil {
		statuses = make(map[Addr]StatusCode)
	}
	return &arbiterClients{
		remoteAddrs: remoteAddrs,
		errors:      errors,
		statuses:    statuses,
	}
}

type ArbiterServer struct {
	// No locking, that should be handled by ArbiterSessionMember.
	ArbiterSessionMember sessionMember
}

func NewArbiterServer(arbiterSessionState sessionMember) *ArbiterServer {
	return &ArbiterServer{arbiterSessionState}
}

// StartListen starts an ArbiterServer (not blocking, uses a goroutine).
// An ArbiterServer by itself isn't really useful; it is intended to be
// used as part of another service where network arbitration is needed,
// so this method isn't really intended to be used except for testing
// and as an example.
func StartListen(a *ArbiterServer, addr Addr) (stop func(), err error) {
	handler := rpc.NewServer()
	handler.Register(a)

	ln, err := net.Listen("tcp", addr.ToStr())
	if err != nil {
		return nil, err
	}

	var conn net.Conn
	stop = func() {
		ln.Close()
		if conn != nil {
			conn.Close()
		}
	}

	go func() {
		for {
			cxn, err := ln.Accept()
			conn = cxn
			if err != nil {
				//log.Printf("listen(%q): %s\n", addr.ToStr(), err)
				break
			}
			go handler.ServeConn(cxn)
		}
	}()
	return stop, nil
}

// RemoteVoteFunc returns a helper func that calls a 'Vote' on a remote
// ArbiterServer. The apsa algorithm requires that each node can call this
// method on all other nodes.
func RemoteVoteFunc() func(addr Addr, sessionID ID) (Addr, ID, StatusCode) {
	return func(addr Addr, sessionID ID) (Addr, ID, StatusCode) {
		var err error
		vote, id, status := ArbiterClient(addr, &err).Vote(sessionID)
		if err != nil {
			return Addr{}, ID(""), StatusDefault
		}
		return vote, id, status
	}
}
