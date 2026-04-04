package classify

// destructiveCommands are commands that are always denied because
// they can cause irreversible damage to the system.
var destructiveCommands = map[string]bool{
	"mkfs":   true,
	"dd":     true,
	"format": true,
	"fdisk":  true,
	"parted": true,
	"shred":  true,
}

// isDestructiveCommand returns true if the command with its arguments
// represents a destructive operation that should be denied.
// "rm" is only destructive with -rf/-fr flags; plain "rm file" is just "ask".
func isDestructiveCommand(cmd string, args []string) bool {
	if destructiveCommands[cmd] {
		return true
	}

	// rm with recursive force flags is destructive
	if cmd == "rm" {
		for _, arg := range args {
			if arg == "-rf" || arg == "-fr" || arg == "-Rf" || arg == "-fR" {
				return true
			}
			// Also catch combined flags like -rfv, -fvr etc
			if len(arg) > 1 && arg[0] == '-' && containsAll(arg[1:], 'r', 'f') {
				return true
			}
		}
		return false
	}

	return false
}

// containsAll checks if a string contains all the specified characters.
func containsAll(s string, chars ...byte) bool {
	for _, c := range chars {
		found := false
		for i := 0; i < len(s); i++ {
			if s[i] == c || s[i] == c-32 { // case-insensitive for R/r, F/f
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// riskyCommands are commands that require user permission because they
// access the network, modify packages, or control system services.
var riskyCommands = map[string]bool{
	// Network
	"curl": true, "wget": true, "ssh": true, "scp": true, "rsync": true,
	// Package managers
	"pip": true, "pip3": true, "npm": true, "npx": true, "yarn": true, "pnpm": true,
	"apt": true, "apt-get": true, "yum": true, "dnf": true, "brew": true,
	"go": true,
	// Containers
	"docker": true, "kubectl": true, "podman": true,
	// System
	"mount": true, "umount": true,
	"kill": true, "pkill": true, "killall": true,
	"reboot": true, "shutdown": true, "poweroff": true, "halt": true,
	"systemctl": true, "service": true,
	// File modification
	"rm": true, "mv": true, "cp": true,
	"mkdir": true, "rmdir": true,
	"chmod": true, "chown": true, "chgrp": true,
	"touch": true, "truncate": true,
	"tee": true,
	"sed": true, "awk": true,
	"nano": true, "vim": true, "vi": true, "emacs": true,
	// Misc risky
	"sleep": true,
}

// isRiskyCommand returns true if the command is risky and should require
// user permission (classified as Ask).
func isRiskyCommand(cmd string) bool {
	return riskyCommands[cmd]
}
