package main

import (
	"io"
	"os/exec"
)

func openTail(fn string) (io.ReadCloser, func() error, error) {
	logInfo("executing tail -n 1000000000 -f %s", fn)
	cmd := exec.Command("tail", "-n", "1000000000", "-f", fn)
	r, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, nil, err
	}

	killer := func() error {
		logInfo("killing tail (pid: %d)", cmd.Process.Pid)
		return cmd.Process.Kill()
	}

	return r, killer, nil
}
