//go:build linux

package platform

// platformShell returns the default shell on Linux.
func platformShell() string { return "/bin/bash" }

// hasPdeathsig returns true on Linux which supports PR_SET_PDEATHSIG.
func hasPdeathsig() bool { return true }
