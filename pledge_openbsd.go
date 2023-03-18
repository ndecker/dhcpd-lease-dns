package main

import (
	"golang.org/x/sys/unix"
)

const (
	Promises     = "stdio inet proc" // proc: kill tail
	ExecPromises = ""
)

func init() {
	runPledge = realRunPledge
}

func realRunPledge() error {
	logInfo("running pledge(\"%s\", \"%s\")", Promises, ExecPromises)
	return unix.Pledge(Promises, ExecPromises)
}
