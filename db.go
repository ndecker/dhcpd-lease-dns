package main

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

type DB struct {
	mu       sync.Mutex
	leases   map[string]Lease // key: lowercase hostname
	ipLeases map[string]Lease // key: ip.String()
}

type Lease struct {
	Starts time.Time
	Ends   time.Time
	// Ethernet       string
	// UID            string
	ClientHostname string
	Abandoned      bool
	IP             net.IP
}

func NewDB() *DB {
	return &DB{
		leases:   make(map[string]Lease),
		ipLeases: make(map[string]Lease),
	}
}

func (db *DB) Add(l Lease) {
	if !l.Prepare() {
		return
	}

	timeRemaining := l.Ends.Sub(time.Now())
	logInfo("adding lease: %s: %s remaining: %s", l.ClientHostname, l.IP.String(), timeRemaining.Round(time.Minute).String())

	db.mu.Lock()
	defer db.mu.Unlock()

	db.leases[strings.ToLower(l.ClientHostname)] = l
	db.ipLeases[l.IP.String()] = l
	db.cleanup()
}

func (db *DB) Lookup(name string) (Lease, bool) {
	db.mu.Lock()
	defer db.mu.Unlock()

	l, ok := db.leases[strings.ToLower(name)]

	if !ok {
		return Lease{}, false
	}

	now := time.Now()
	if l.Starts.After(now) {
		return Lease{}, false
	}

	if l.Ends.Before(now) {
		return Lease{}, false
	}
	return l, true
}

func (db *DB) LookupIP(ip net.IP) (Lease, bool) {
	db.mu.Lock()
	defer db.mu.Unlock()

	l, ok := db.ipLeases[ip.String()]

	if !ok {
		return Lease{}, false
	}

	now := time.Now()
	if l.Starts.After(now) {
		return Lease{}, false
	}

	if l.Ends.Before(now) {
		return Lease{}, false
	}
	return l, true
}

func (db *DB) cleanup() {
	// must be called with lock held

	now := time.Now()
	for k, l := range db.leases {
		if l.Ends.Before(now) {
			logDebug("deleting expired lease: %s", k)
			delete(db.leases, k)
		}
	}

	for k, l := range db.ipLeases {
		if l.Ends.Before(now) {
			logDebug("deleting expired lease: %s", k)
			delete(db.ipLeases, k)
		}
	}
}

// Prepare Lease to be added to DB. Returns false if not suitable.
func (l *Lease) Prepare() bool {
	if l.Abandoned {
		logDebug("ignoring abandoned lease: %v", l)
		return false
	}

	if l.Ends.Before(time.Now()) {
		logDebug("ignoring expired lease: %v", l)
		return false
	}

	if l.ClientHostname == "" {
		l.ClientHostname = fmt.Sprintf("dhcp-%d-%d-%d-%d", l.IP[0], l.IP[1], l.IP[2], l.IP[3])
	}
	return true
}

func (l Lease) String() string {
	return fmt.Sprintf("%s: %s (ends %s)",
		l.ClientHostname, l.IP.String(),
		l.Ends.Sub(time.Now()).Round(time.Second).String())
}
