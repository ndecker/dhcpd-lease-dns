package main

import (
	"fmt"
	"net"
	"strings"

	"github.com/miekg/dns"
)

const InAddrArpa = "in-addr.arpa."

func startDns(db *DB, domain string, port int) error {
	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}

	started := make(chan struct{})
	listenError := make(chan error)

	mux := dns.NewServeMux()

	mux.HandleFunc(domain, dnsHandler(lookupName(db, domain)))
	mux.HandleFunc(InAddrArpa, dnsHandler(lookupIp(db, domain)))

	server := dns.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Net:     "udp",
		Handler: mux,
	}
	server.NotifyStartedFunc = func() {
		logInfo("DNS server started")
		close(started)
	}

	go func() {
		logInfo("starting DNS server on %s", server.Addr)
		err := server.ListenAndServe()
		listenError <- err
		close(listenError)
	}()

	select {
	case le := <-listenError:
		return le
	case <-started:
		go func() {
			le := <-listenError
			logFatal("listen dns: %s", le)
		}()
		return nil
	}

}

type DnsLookup func(query string, typ uint16) []dns.RR

func lookupName(db *DB, domain string) DnsLookup {
	return func(query string, typ uint16) []dns.RR {
		if typ != dns.TypeA {
			return nil
		}

		if !strings.HasSuffix(query, domain) {
			logInfo("dns query does not match configured domain: %s / %s", query, domain)
			return nil
		}

		hostname := strings.TrimSuffix(query, domain)
		hostname = strings.TrimSuffix(hostname, ".")

		lease := db.Lookup(hostname)
		if lease == nil {
			logDebug("hostname not found: '%s'", hostname)
			return nil
		}

		rr := &dns.A{
			Hdr: dns.RR_Header{
				Name:   fmt.Sprintf("%s.%s", lease.ClientHostname, domain),
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			A: lease.IP,
		}

		return []dns.RR{rr}
	}
}

func lookupIp(db *DB, domain string) DnsLookup {
	return func(query string, typ uint16) []dns.RR {
		if typ != dns.TypePTR {
			return nil
		}

		if !strings.HasSuffix(query, "."+InAddrArpa) {
			logInfo("unexpected query: %s", query)
			return nil
		}

		ipStr := strings.TrimSuffix(query, "."+InAddrArpa)
		ipParts := strings.Split(ipStr, ".")
		for i, j := 0, len(ipParts)-1; i < j; i, j = i+1, j-1 {
			ipParts[i], ipParts[j] = ipParts[j], ipParts[i]
		}
		ipStr = strings.Join(ipParts, ".")

		ip := net.ParseIP(ipStr)
		if ip == nil {
			logInfo("cannot parse ip: %s", ipStr)
			return nil
		}

		lease := db.LookupIP(ip)
		if lease == nil {
			logDebug("ip not found: '%s'", ip)
			return nil
		}

		ptr := &dns.PTR{
			Hdr: dns.RR_Header{
				Name:   query,
				Rrtype: dns.TypePTR,
				Class:  dns.ClassINET,
				Ttl:    0,
			},
			Ptr: fmt.Sprintf("%s.%s", lease.ClientHostname, domain),
		}

		return []dns.RR{ptr}
	}
}

func dnsHandler(lookup DnsLookup) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		logDebug("DNS Request: %+v", r)

		m := &dns.Msg{}
		m.SetReply(r)

		for i, q := range r.Question {
			query := q.Name
			logDebug("query %d: %s\n", i, query)

			answer := lookup(query, r.Question[0].Qtype)
			logDebug("answer: %v", answer)
			m.Answer = append(m.Answer, answer...)
		}

		err := w.WriteMsg(m)
		if err != nil {
			logInfo("DNS write error: %w", err)
		}
	}
}
