package main

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const BufSize = 4096 // must fit longest dhcpd record

func parseLeases(r io.ReadCloser, db *DB) error {
	lp := leaseParser{}
	buf := make([]byte, BufSize)
	for {
		buf = buf[0:cap(buf)] // restore full buf

		n, err := r.Read(buf)
		if err != nil {
			return fmt.Errorf("error reading leases: %w", err)
		}

		buf = buf[0:n] // limit buf to read size
		lp.AddData(buf)

		for {
			l, ok := lp.ParseLease()
			if !ok {
				logDebug("parser: no lease: %s", lp.data[lp.pos:lp.pos+10])
				break
			}
			logDebug("parser: lease found: %+v", l)
			db.Add(l)
		}
	}
}

type leaseParser struct {
	data []byte
	pos  int
}

func (lp *leaseParser) AddData(data []byte) {
	logDebug("parser: AddData(%d)", len(data))
	lp.data = append(lp.data[lp.pos:], data...)
	lp.pos = 0
}

// ParseLease parses one lease and returns it. If not enough data is present, null is returned.
func (lp *leaseParser) ParseLease() (l Lease, ok bool) {
	startPos := lp.pos
	defer func() {
		err := recover()
		if err == io.EOF {
			logDebug("parser: recover EOF: %d => %d", lp.pos, startPos)
			lp.pos = startPos // restore start position
			ok = false
			return
		} else if err != nil {
			panic(err)
		}
	}()

	for { // parse lease header
		lp.skipWS()
		if !lp.Consume([]byte("lease")) {
			lp.discardToNL()
			continue
		}

		lp.skipWS()
		l.IP, ok = lp.ParseIP()
		if !ok {
			lp.discardToNL()
			continue
		}
		lp.skipWS()

		if !lp.Consume([]byte("{")) {
			lp.discardToNL()
			continue
		}

		break // success
	}

	logDebug("parser: found lease: %s\n", l.IP)

	for {
		lp.skipWS()
		if lp.Consume([]byte("}")) {
			logDebug("parser: lease done")
			return l, true // keep lp.pos
		}

		if lp.Consume([]byte("starts")) {
			lp.skipWS()
			timeBytes := lp.CollectUntil(';')

			ts, ok := parseTimestamp(timeBytes)
			if !ok {
				lp.discardToNL()
				continue
			}

			l.Starts = ts
			continue
		}

		if lp.Consume([]byte("ends")) {
			lp.skipWS()
			timeBytes := lp.CollectUntil(';')

			ts, ok := parseTimestamp(timeBytes)
			if !ok {
				lp.discardToNL()
				continue
			}

			l.Ends = ts
			continue
		}

		if lp.Consume([]byte("hardware ethernet")) {
			lp.skipWS()
			_ = lp.CollectUntil(';')
			// l.Ethernet = string(eth)
			continue
		}

		if lp.Consume([]byte("uid")) {
			lp.skipWS()
			_ = lp.CollectUntil(';')
			// l.UID = string(uid)
			continue
		}

		if lp.Consume([]byte("client-hostname")) {
			lp.skipWS()
			hostname := string(lp.CollectUntil(';'))
			l.ClientHostname = strings.Trim(hostname, "\"")

			continue
		}

		if lp.Consume([]byte("abandoned")) {
			lp.skipWS()
			lp.Consume([]byte(";"))
			l.Abandoned = true
			continue
		}

		// nothing matches
		lp.discardToNL()
		continue
	}
}

func (lp *leaseParser) Consume(expected []byte) bool {
	ok := bytes.HasPrefix(lp.data[lp.pos:], expected)
	if ok {
		lp.pos += len(expected)
	}
	return ok
}

func (lp *leaseParser) CollectUntil(sep byte) []byte {
	i := 0
	for {
		if lp.pos+i >= len(lp.data) {
			panic(io.EOF)
		}
		if lp.data[lp.pos+i] == sep {
			break
		}
		i++
	}

	data := lp.data[lp.pos : lp.pos+i]
	lp.pos += i + 1
	return data
}

func (lp *leaseParser) CollectBytes(bs []byte) []byte {
	i := 0
	for {
		if lp.pos+i >= len(lp.data) {
			panic(io.EOF)
		}
		if bytes.IndexByte(bs, lp.data[lp.pos+i]) == -1 {
			break
		}
		i++
	}

	data := lp.data[lp.pos : lp.pos+i]
	lp.pos += i
	return data
}

func (lp *leaseParser) ParseIP() (net.IP, bool) {
	ipBytes := lp.CollectBytes([]byte("0123456789."))
	ip := net.ParseIP(string(ipBytes))
	return ip, ip != nil
}

func (lp *leaseParser) skipWS() {
	_ = lp.CollectBytes([]byte(" \n\t"))
}

func (lp *leaseParser) discardToNL() {
	disc := lp.CollectUntil('\n')
	logInfo("discarding: '%s'", disc)
}

func parseTimestamp(ts []byte) (time.Time, bool) {
	if len(ts) < 2 {
		return time.Time{}, false
	}
	ts = ts[2:] // skip weekday ("2 ")

	const TimeFormat = "2006/01/02 15:04:05 UTC" // 2022/04/24 17:27:07 UTC
	timestamp, err := time.Parse(TimeFormat, string(ts))
	if err != nil {
		logInfo("cannot parse timestamp \"%s\": %w", string(ts), err)
		return time.Time{}, false
	}
	return timestamp, true
}
