// Package classify implements bash command security classification.
// Commands are classified using AST parsing (not regex) via mvdan.cc/sh
// to correctly handle pipes, command substitution, subshells, and quoting.
// Unparseable commands fail closed (classified as Ask).
package classify

import (
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// ClassificationResult indicates how a bash command should be treated.
type ClassificationResult int

const (
	// ClassifySafe means the command is safe and can auto-execute.
	ClassifySafe ClassificationResult = iota
	// ClassifyReadOnly means the command only reads data and can auto-execute.
	ClassifyReadOnly
	// ClassifyAsk means the command requires user permission.
	ClassifyAsk
	// ClassifyDeny means the command is blocked (destructive/dangerous).
	ClassifyDeny
)

// String returns the string representation of a ClassificationResult.
func (c ClassificationResult) String() string {
	switch c {
	case ClassifySafe:
		return "safe"
	case ClassifyReadOnly:
		return "readonly"
	case ClassifyAsk:
		return "ask"
	case ClassifyDeny:
		return "deny"
	default:
		return "ask"
	}
}

// ClassifyBashCommand classifies a bash command string into a security category.
// It parses the command using mvdan.cc/sh AST parser and evaluates each
// individual command in the pipeline/chain against the security rules.
// Returns the worst (most restrictive) classification found plus a reason string.
func ClassifyBashCommand(command string) (ClassificationResult, string) {
	command = strings.TrimSpace(command)
	if command == "" {
		return ClassifyAsk, "empty command"
	}

	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		return ClassifyAsk, "unparseable command"
	}

	// Check for redirections that write to files at the top level
	hasWriteRedirect := false
	syntax.Walk(file, func(node syntax.Node) bool {
		if redirect, ok := node.(*syntax.Redirect); ok {
			// > >> >& are write redirects
			if redirect.Op == syntax.RdrOut || redirect.Op == syntax.AppOut ||
				redirect.Op == syntax.RdrAll || redirect.Op == syntax.AppAll {
				hasWriteRedirect = true
			}
		}
		return true
	})

	// Extract all commands with their arguments from the AST
	entries := extractCommandEntries(file)

	if len(entries) == 0 {
		return ClassifyAsk, "no commands found"
	}

	worst := ClassifySafe
	var reason string

	for _, entry := range entries {
		var result ClassificationResult
		var r string

		if entry.name == "" {
			// Non-literal command name (variable expansion, etc.)
			result = ClassifyAsk
			r = "dynamic command name"
		} else if isDestructiveCommand(entry.name, entry.args) {
			result = ClassifyDeny
			r = "destructive command: " + entry.name
		} else if isRiskyCommand(entry.name) {
			result = ClassifyAsk
			r = "risky command: " + entry.name
		} else if isReadOnlyCommand(entry.name, entry.args) {
			result = ClassifyReadOnly
			r = "read-only command: " + entry.name
		} else {
			result = ClassifyAsk
			r = "unrecognized command: " + entry.name
		}

		if result > worst {
			worst = result
			reason = r
		}
	}

	// Write redirections escalate read-only commands to ask
	if hasWriteRedirect && worst < ClassifyAsk {
		worst = ClassifyAsk
		reason = "output redirection"
	}

	return worst, reason
}

// commandEntry represents a parsed command with its name and arguments.
type commandEntry struct {
	name string
	args []string
}

// extractCommandEntries walks the AST and extracts all command names with their arguments.
func extractCommandEntries(file *syntax.File) []commandEntry {
	var entries []commandEntry

	syntax.Walk(file, func(node syntax.Node) bool {
		switch x := node.(type) {
		case *syntax.CallExpr:
			if len(x.Args) == 0 {
				return true
			}
			name := literalValue(x.Args[0])

			var args []string
			for i := 1; i < len(x.Args); i++ {
				args = append(args, literalValue(x.Args[i]))
			}

			// Handle sudo: classify the actual command
			if name == "sudo" && len(args) > 0 {
				// Skip sudo flags to find the actual command
				actualIdx := 0
				for actualIdx < len(args) && strings.HasPrefix(args[actualIdx], "-") {
					actualIdx++
				}
				if actualIdx < len(args) {
					actualName := args[actualIdx]
					var actualArgs []string
					if actualIdx+1 < len(args) {
						actualArgs = args[actualIdx+1:]
					}
					entries = append(entries, commandEntry{name: actualName, args: actualArgs})
					return true
				}
			}

			entries = append(entries, commandEntry{name: name, args: args})
		case *syntax.DeclClause:
			// export, declare, local, etc. are safe
			entries = append(entries, commandEntry{name: x.Variant.Value, args: nil})
		}
		return true
	})

	return entries
}

// ExtractCommands is an alias for ExtractCommandNames for API compatibility.
var ExtractCommands = ExtractCommandNames

// ExtractCommandNames extracts all command names from a bash command string.
// Returns a slice of command name strings. Non-literal command names
// (e.g., variable expansions) are returned as empty strings.
func ExtractCommandNames(command string) []string {
	parser := syntax.NewParser(syntax.Variant(syntax.LangBash))
	file, err := parser.Parse(strings.NewReader(command), "")
	if err != nil {
		return nil
	}

	var names []string
	syntax.Walk(file, func(node syntax.Node) bool {
		if call, ok := node.(*syntax.CallExpr); ok && len(call.Args) > 0 {
			names = append(names, literalValue(call.Args[0]))
		}
		return true
	})
	return names
}

// literalValue extracts a literal string from a shell Word.
// If the Word contains any non-literal parts (expansions, substitutions),
// it returns an empty string.
func literalValue(w *syntax.Word) string {
	var sb strings.Builder
	for _, part := range w.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			sb.WriteString(p.Value)
		case *syntax.SglQuoted:
			sb.WriteString(p.Value)
		case *syntax.DblQuoted:
			// Check if all parts inside double quotes are literal
			for _, inner := range p.Parts {
				if lit, ok := inner.(*syntax.Lit); ok {
					sb.WriteString(lit.Value)
				} else {
					return ""
				}
			}
		default:
			return ""
		}
	}
	return sb.String()
}
