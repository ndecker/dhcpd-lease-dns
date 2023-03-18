package main

import (
	"flag"
	"github.com/sevlyar/go-daemon"
	"log"
	"log/syslog"
	"os"
	"path/filepath"
)

var (
	flagDaemon = flag.Bool("daemon", false, "daemonize process")

	flagPidFile = flag.String("daemon.pidfile", "", "pid file name")
	// flagLogFile     = flag.String("daemon.logfile", "", "logfile file name")
	// flagLogFilePerm = flag.Int("daemon.logfileperm", 0640, "logfile permissions")
	flagChroot = flag.String("daemon.chroot", "", "chroot directory")
	flagSyslog = flag.Bool("daemon.syslog", true, "log to syslog")
)

func daemonize(f func()) {
	if !*flagDaemon {
		f()
		return
	}

	cntx := &daemon.Context{
		PidFileName: *flagPidFile,
		PidFilePerm: 0,
		//LogFileName: *flagLogFile,
		//LogFilePerm: os.FileMode(*flagLogFilePerm),
		Chroot: *flagChroot,
	}

	d, err := cntx.Reborn()
	if err != nil {
		logFatal("unable to daemonize: %s", err)
	}
	if d != nil {
		return
	}

	defer func() {
		err := cntx.Release()
		if err != nil {
			logFatal("cannot release daemon context: %s", err)
		}
	}()

	if *flagSyslog {
		sysl, err := syslog.New(syslog.LOG_INFO|syslog.LOG_USER, filepath.Base(os.Args[0]))
		if err != nil {
			logFatal("cannot open syslog: %s", err)
		}

		logger = log.New(sysl, "", log.Flags())
	}

	f()
}
