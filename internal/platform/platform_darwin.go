//go:build darwin

package platform

// platformShell returns the default shell on macOS.
func platformShell() string { return "/bin/zsh" }

// hasPdeathsig returns false on macOS which does not support PR_SET_PDEATHSIG.
func hasPdeathsig() bool { return false }
