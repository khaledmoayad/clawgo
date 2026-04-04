package classify

// readOnlyCommands is the set of commands that are safe to auto-execute
// because they only read data and don't modify the system.
// This matches the TypeScript bashClassifier.ts allowlist.
var readOnlyCommands = map[string]bool{
	// File listing / info
	"ls": true, "cat": true, "head": true, "tail": true, "wc": true,
	"file": true, "stat": true, "du": true, "df": true,
	"tree": true, "column": true, "nl": true,

	// Search
	"grep": true, "egrep": true, "fgrep": true, "find": true,
	"rg": true, "fd": true, "ag": true, "ack": true,

	// Text processing (read-only)
	"sort": true, "uniq": true, "tr": true, "cut": true, "paste": true,
	"comm": true, "diff": true, "cmp": true, "jq": true, "yq": true,

	// System info
	"which": true, "whoami": true, "hostname": true, "uname": true,
	"date": true, "pwd": true, "id": true, "groups": true,
	"env": true, "printenv": true, "locale": true,
	"tput": true, "tty": true, "stty": true,

	// Shell builtins (read-only)
	"echo": true, "printf": true, "test": true, "true": true, "false": true,
	"cd": true, "pushd": true, "popd": true, "dirs": true,
	"type": true, "command": true, "hash": true,
	"help": true,

	// Math / path
	"seq": true, "expr": true, "basename": true, "dirname": true,
	"realpath": true, "readlink": true,

	// Checksums
	"md5sum": true, "sha1sum": true, "sha256sum": true, "sha512sum": true, "cksum": true,

	// Pager / display
	"less": true, "more": true, "bat": true,
	"man": true, "info": true,

	// Binary inspection
	"od": true, "hexdump": true, "xxd": true, "strings": true,

	// Misc safe
	"xargs": true,
	"mattn": true,
}

// gitReadOnlySubcommands lists git subcommands that only read data.
var gitReadOnlySubcommands = map[string]bool{
	"status":   true,
	"log":      true,
	"diff":     true,
	"branch":   true,
	"show":     true,
	"tag":      true,
	"remote":   true,
	"describe": true,
	"rev-parse": true,
	"ls-files": true,
	"ls-tree":  true,
	"blame":    true,
	"shortlog": true,
	"reflog":   true,
}

// isReadOnlyCommand returns true if the command is known to be read-only.
// For git, it checks whether the subcommand is in the safe list.
func isReadOnlyCommand(cmd string, args []string) bool {
	if cmd == "git" {
		if len(args) == 0 {
			return true // bare "git" shows help
		}
		return gitReadOnlySubcommands[args[0]]
	}
	return readOnlyCommands[cmd]
}
