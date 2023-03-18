package main

import (
	"fmt"
	"strings"

	"github.com/miekg/dns"
)

// TODO: in-addr.arpa

func startDns(db *DB, domain string, port int) error {
	if !strings.HasSuffix(domain, ".") {
		domain = domain + "."
	}

	started := make(chan struct{})
	listenError := make(chan error)

	mux := dns.NewServeMux()

	mux.HandleFunc(domain, dnsHandler(db, domain))

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

func dnsHandler(db *DB, domain string) func(w dns.ResponseWriter, r *dns.Msg) {
	return func(w dns.ResponseWriter, r *dns.Msg) {
		logDebug("DNS Request: %+v", r)

		m := &dns.Msg{}
		m.SetReply(r)

		if len(r.Question) < 1 {
			return
		}

		query := r.Question[0].Name
		// logDebug("query: %s\n", query)

		if !strings.HasSuffix(query, domain) {
			logFatal("dns query does not match configured domain: %s / %s", query, domain)
		}

		hostname := strings.TrimSuffix(query, domain)
		hostname = strings.TrimSuffix(hostname, ".")

		lease, ok := db.Lookup(hostname)
		if !ok {
			logDebug("hostname not found: '%s'", hostname)
			err := w.WriteMsg(m)
			if err != nil {
				logInfo("cannot send reply: %s", err)
			}
			return
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

		switch r.Question[0].Qtype {
		case dns.TypeA:
			m.Answer = append(m.Answer, rr)
			// m.Extra = append(m.Extra, t)
		}

		logDebug("%v\n", m.String())

		err := w.WriteMsg(m)
		if err != nil {
			logInfo("DNS write error: %w", err)
		}
	}
}
