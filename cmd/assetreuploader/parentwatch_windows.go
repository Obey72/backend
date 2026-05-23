//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// returns true if the windows process with this pid is still running
// uses openprocess with the limited query right so we don't need elevated perms
// followed by getexitcodeprocess and the still_active sentinel
func isProcessAlive(pid int) bool {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	const STILL_ACTIVE = 259

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")
	closeHandle := kernel32.NewProc("CloseHandle")
	getExitCode := kernel32.NewProc("GetExitCodeProcess")

	handle, _, _ := openProcess.Call(uintptr(PROCESS_QUERY_LIMITED_INFORMATION), 0, uintptr(pid))
	if handle == 0 {
		return false
	}
	defer closeHandle.Call(handle)

	var exitCode uint32
	ret, _, _ := getExitCode.Call(handle, uintptr(unsafe.Pointer(&exitCode)))
	if ret == 0 {
		return false
	}
	return exitCode == STILL_ACTIVE
}

// waitForProcessExit blocks until the given pid exits, returns false if we
// couldnt open it (already dead or not visible) so the caller can decide
// whether to keep going, on windows we just sync on the process handle
// itself which the kernel signals the instant the process leaves
func waitForProcessExit(pid int) bool {
	const SYNCHRONIZE = 0x00100000
	const INFINITE = 0xFFFFFFFF

	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	openProcess := kernel32.NewProc("OpenProcess")
	closeHandle := kernel32.NewProc("CloseHandle")
	waitForSingle := kernel32.NewProc("WaitForSingleObject")

	handle, _, _ := openProcess.Call(uintptr(SYNCHRONIZE), 0, uintptr(pid))
	if handle == 0 {
		return false
	}
	defer closeHandle.Call(handle)

	waitForSingle.Call(handle, uintptr(INFINITE))
	return true
}
