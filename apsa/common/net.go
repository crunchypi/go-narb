/*
This file contains some common tools for apsa: Addr and ID.
Addr is a substitute for the native Go Addr with helper
methods, while ID will be shared type of SessionMembers and
the RPC implementation.
*/
package common

import (
	"fmt"
	"math/rand"
	"time"
)

// Addr is a substitute for the native Go Addr.
type Addr struct {
	IP   string
	Port string
}

// ToStr convert Addr into a "ip:port" string.
func (a *Addr) ToStr() string {
	return fmt.Sprintf("%s:%s", a.IP, a.Port)
}

// Comp checks whether or not Addr == other.
func (a *Addr) Comp(other Addr) bool {
	return a.IP == other.IP && a.Port == other.Port
}

// In checks whether or not Addr is in a []Addr
func (a *Addr) In(others []Addr) bool {
	for _, other := range others {
		if a.Comp(other) {
			return true
		}
	}
	return false
}

// FilterFrom filters Addr from the accepted slice.
func (a *Addr) FilterFrom(others []Addr) []Addr {
	r := make([]Addr, 0, len(others))
	for _, addr := range others {
		if !a.Comp(addr) {
			r = append(r, addr)
		}
	}
	return r
}

// ID is a subtype of string, representing an ID.
type ID string

// NewRandID creates a random ID with length n. It uses a mix
// of upper- and lower-case english letters as well as digits.
func NewRandID(n int) ID {
	rand.Seed(time.Now().UnixNano())
	c := "abcdefghijklmnopqrstuvwxyz"
	c += "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	c += "0123456789"
	characters := []rune(c)
	id := make([]rune, n)
	for i := 0; i < n; i++ {
		id[i] = characters[rand.Intn(len(characters))]
	}
	return ID(id)
}

func (id *ID) ToIDStr() string { return string(*id) }
