/*
This file contains some common tools for apsa. Specifically, arbiter
data which will be shared between arbiter session members and the
RPC implementation.
*/
package common

import "time"

// ArbiterData represents an arbiter found through network consensus.
type ArbiterData struct {
	ID         ID
	Addr       Addr
	Expiration time.Time
}

// NewArbiterData is a helper for setting up ArbiterData.
func NewArbiterData(a Addr, id ID, expiration time.Time) ArbiterData {
	return ArbiterData{
		ID:         id,
		Addr:       a,
		Expiration: expiration,
	}
}

// Expired is a helper for checking whether or not an arbiter expired.
func (a *ArbiterData) Expired() bool {
	return time.Now().After(a.Expiration)
}
