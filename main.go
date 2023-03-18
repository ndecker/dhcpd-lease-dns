package main

import (
	_ "embed"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
)

type LogFunc func(f string, args ...any)

var (
	runPledge func() error = nil // only on OpenBSD

	flagDomain     = flag.String("domain", "dhcp.local", "local domain for replies")
	flagDnsPort    = flag.Int("dns-port", 5333, "DNS UDP port") // 5353 is mdns
	flagLeasesFile = flag.String("dhcpd-leases", "/var/db/dhcpd.leases", "dhcpd leases file")

	flagDebug   = flag.Bool("debug", false, "debug logging")
	flagQuiet   = flag.Bool("quiet", false, "quiet normal logging")
	flagLicense = flag.Bool("license", false, "show license")
	flagReadme  = flag.Bool("readme", false, "show Readme.md")

	logger           = log.Default()
	logInfo  LogFunc = dolog // need indirection becase logger can be changed later
	logDebug LogFunc = discard
	logFatal LogFunc = dofatal

	exitHooks []func() error

	//go:embed LICENSE
	license string

	//go:embed README.md
	readme string
)

func run() {
	logInfo("starting dhcpd-leases-dns version %s", version())

	signalExit := make(chan os.Signal)
	signal.Notify(signalExit, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)
	go func() {
		sig := <-signalExit
		logFatal("signal caught: %s", sig)
	}()

	db := NewDB()

	dhcpLeases, closeDhcpLeases, err := openTail(*flagLeasesFile)
	if err != nil {
		logFatal("tailing dhcpd.leases: %s", err)
	}
	defer func() { _ = dhcpLeases.Close() }()
	exitHooks = append(exitHooks, closeDhcpLeases)

	err = startDns(db, *flagDomain, *flagDnsPort)
	if err != nil {
		logFatal("starting dns: %s", err)
	}

	// run pledge after opening tail and listening DNS
	if runPledge != nil {
		err := runPledge()
		if err != nil {
			logFatal("pledge: %s", err)
		}
	}

	err = parseLeases(dhcpLeases, db)
	if err != nil {
		logFatal("parse leases: %s", err)
	}
}

func main() {
	flag.Parse()

	if *flagLicense {
		fmt.Println(license)
		return
	}

	if *flagReadme {
		fmt.Println(readme)
		return
	}

	if *flagDebug {
		logDebug = dolog
	}

	if *flagQuiet {
		logInfo = discard
	}

	daemonize(run)

}

func discard(string, ...any)      {}
func dolog(f string, args ...any) { logger.Printf(f, args...) }
func dofatal(f string, args ...any) {
	logger.Printf(f, args...)

	for _, h := range exitHooks {
		err := h()
		if err != nil {
			logger.Print(err)
		}
	}
	os.Exit(1)
}

func version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "no version"
	}
	return bi.Main.Version
}
