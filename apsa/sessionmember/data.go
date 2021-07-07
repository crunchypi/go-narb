/*
This file contains data which sessionmembers store internally, i.e state of
sessions as well as helper methods for those state types.

Further description and justification:
	- voteTableItem	(keeping track of who voted for who).
	- sessionData 	(type used for each sessions at a time).
	- Funcs			(decouling from network comm implementation).


*/
package sessionmember

import (
	"math/rand"
	"time"

	"github.com/crunchypi/go-narb/apsa/common"
)

// Addr abbreviates common.Addr.
type Addr = common.Addr

// ID abbreviates common.ID.
type ID = common.ID

// StatusCode abbreviates common.StatusCode.
type StatusCode = common.StatusCode

// ArbiterData common.ArbiterData
type ArbiterData = common.ArbiterData

// Abbreviations.
var newRandID = common.NewRandID
var newArbiterData = common.NewArbiterData

// voteTableItem represents a single arbiter _vote_. It is intended to be used
// in a vote table, with the following structure: map[X]Y, where X = Addr and
// Y = voteTableItem. X is the address of who voted, while Y is the vote itself,
// along with some additional data.
type voteTableItem struct {
	// Marker for set/unset voteTableItem.
	voted bool
	// Who a voter voted for.
	vote Addr
	// The ID given by the voter. This will make it easy for any SessionMember
	// to know whether or not they are the arbiter, as this ID will be 'localID'
	// (field of type sessionData, defined further down).
	voteID ID
}

// sessionData represents the local SessionMember data for a particular arbiter
// vote session. This will last until either (A) sessionExpiration expired, or
// (B) a new session is initiated.
type sessionData struct {
	// ID for a particular session, provided by the session initiator. This
	// sessions to actual votes.
	sessionID ID
	// When the session expires.
	sessionExpiration time.Time
	// Unique ID for a local sessionMember, given out while voting. When an
	// arbiter consensus is reached, this can be checked against voteID in
	// voteTableItems (stored in voteTable of sessionData), to know if the
	// local SessionMember is the arbiter.
	localID ID
	// Who the local SessionMember votes for.
	localVote Addr
	// Key=Who voted, Val= Who Key voted for.
	voteTable map[Addr]voteTableItem
}

// newSessionDataArgs is solely used as argiments to newSessionData().
type newSessionDataArgs struct {
	sessionID       ID
	voteOpt         []Addr
	sessionDuration time.Duration
}

// newSessionData configures a new sessionData instance.
func newSessionData(args newSessionDataArgs) sessionData {
	m := make(map[Addr]voteTableItem, len(args.voteOpt))
	for _, voteOpt := range args.voteOpt {
		m[voteOpt] = voteTableItem{}
	}
	rand.Seed(time.Now().UnixNano())
	return sessionData{
		sessionID:         args.sessionID,
		sessionExpiration: time.Now().Add(args.sessionDuration),
		localID:           newRandID(10),
		localVote:         args.voteOpt[rand.Intn(len(args.voteOpt))],
		voteTable:         m,
	}
}

// expired returns true if the session expired.
func (s *sessionData) expired() bool {
	return time.Now().After(s.sessionExpiration)
}

// addVote adds an arbiter vote to the internal voteTable.
func (s *sessionData) addVote(voteBy, vote Addr, voteID ID) {
	s.voteTable[voteBy] = voteTableItem{
		voted:  true,
		vote:   vote,
		voteID: voteID,
	}
}

// tallyVotes converts the internal voteTable into a map where keys are Addr in
// string format, while vals represents how many votes those Addr were voted for.
func (s *sessionData) tallyVotes() map[string]int {
	tally := make(map[string]int, len(s.voteTable))
	for _, item := range s.voteTable {
		if !item.voted {
			continue
		}
		votes, _ := tally[item.vote.ToStr()]
		votes++
		tally[item.vote.ToStr()] = votes
	}
	return tally
}

// voteConsensus checks if the voteTable has an Addr that has most votes.
func (s *sessionData) voteConsensus() bool {
	_, consensus := maxMapVal(s.tallyVotes())
	return consensus
}

// nextArbiter checks if the internal voteTable has an Addr that has most votes
// and returns a valid arbiter and true if that is the case, else and empty
// arbiter and false is returned.
func (s *sessionData) nextArbiter(arbiterDur time.Duration) (ArbiterData, bool) {
	bestKey, consensus := maxMapVal(s.tallyVotes())
	if !consensus {
		return ArbiterData{}, consensus
	}

	// Simply indexing voteTable with bestKey isn't correct because:
	// 1) bestKey   = Addr.
	// 2) votetable = map[Addr]tableItem = map[whoVoted]arbiterCandidate.
	//
	// So that would give the voter addr and not the arbiter vote itself. The
	// block below finds the first tableItem where the vote == bestKey.
	var arbiter ArbiterData
	for _, item := range s.voteTable {
		if item.vote.ToStr() == bestKey {
			expiration := time.Now().Add(arbiterDur)
			arbiter = newArbiterData(item.vote, item.voteID, expiration)
			break
		}
	}
	return arbiter, consensus
}

// voteCompleted checks if all votes were accounted for (i.e collected).
func (s *sessionData) voteCompleted() bool {
	for _, item := range s.voteTable {
		if !item.voted {
			return false
		}
	}
	return true
}

// Funcs contains funcs that are needed for a SessionMember to contact other
// SessionMembers. This separates the implementation of any network calls.
type Funcs struct {
	// RemoteVoteFunc represents an implementation that lets a local
	// SessionMember to call the SessionMember.Vote() method of a remote node.
	RemoteVoteFunc func(remoteAddr Addr, sessionID ID) (Addr, ID, StatusCode)
}

// ok varifies that a Funcs instance is valid.
func (f *Funcs) ok() bool {
	if f.RemoteVoteFunc == nil {
		return false
	}
	return true
}
