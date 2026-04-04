//go:build windows

package platform

// platformShell returns the default shell on Windows.
func platformShell() string { return "cmd.exe" }

// hasPdeathsig returns false on Windows which does not support PR_SET_PDEATHSIG.
func hasPdeathsig() bool { return false }
