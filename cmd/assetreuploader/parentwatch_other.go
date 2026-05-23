//go:build !windows

package main

import (
	"os"
	"syscall"
	"time"
)

// unix path uses signal 0 which is a no-op delivery that still triggers the
// kernel permission and existence checks so a nil error means the process is
// alive and reachable
func isProcessAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return p.Signal(syscall.Signal(0)) == nil
}

// waitForProcessExit polls the parent every 200ms, posix has no blocking
// wait on an arbitrary pid (only on children of the current process) so
// tight-polling is the cleanest portable option, returns true once the
// parent is gone
func waitForProcessExit(pid int) bool {
	for {
		if !isProcessAlive(pid) {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
}
