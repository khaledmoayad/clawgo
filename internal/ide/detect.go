package ide

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// IDEType represents the type of IDE detected.
type IDEType string

const (
	// VSCode indicates a Visual Studio Code instance.
	VSCode IDEType = "vscode"
	// JetBrains indicates a JetBrains IDE instance (GoLand, IntelliJ, etc.).
	JetBrains IDEType = "jetbrains"
	// Cursor indicates a Cursor IDE instance.
	Cursor IDEType = "cursor"
	// Windsurf indicates a Windsurf IDE instance.
	Windsurf IDEType = "windsurf"
	// Zed indicates a Zed editor instance.
	Zed IDEType = "zed"
	// Unknown indicates no recognized IDE was detected.
	Unknown IDEType = "unknown"
)

// IDEInfo holds information about the detected IDE.
type IDEInfo struct {
	Type    IDEType
	Name    string
	Version string
	PID     int
}

// knownVSCodeBinaries are process names that indicate VS Code.
var knownVSCodeBinaries = []string{
	"code",
	"code-insiders",
	"code-oss",
	"codium",
}

// knownJetBrainsBinaries are process names that indicate JetBrains IDEs.
var knownJetBrainsBinaries = []string{
	"idea",
	"idea64",
	"goland",
	"goland64",
	"webstorm",
	"webstorm64",
	"pycharm",
	"pycharm64",
	"phpstorm",
	"phpstorm64",
	"clion",
	"clion64",
	"rider",
	"rider64",
	"rubymine",
	"rubymine64",
	"datagrip",
	"datagrip64",
}

// knownCursorBinaries are process names that indicate Cursor IDE.
var knownCursorBinaries = []string{
	"cursor",
	"cursor-insiders",
}

// knownWindsurfBinaries are process names that indicate Windsurf IDE.
var knownWindsurfBinaries = []string{
	"windsurf",
	"windsurf-insiders",
}

// knownZedBinaries are process names that indicate Zed editor.
var knownZedBinaries = []string{
	"zed",
}

// DetectIDE detects the running IDE by checking environment variables
// (fastest) and falling back to ancestor process scanning.
func DetectIDE() IDEInfo {
	// Check env vars first (fastest path)
	if info, ok := detectFromEnv(); ok {
		return info
	}

	// Fallback: scan ancestor processes
	return detectFromProcesses()
}

// detectFromEnv checks environment variables set by IDEs.
// Cursor and Windsurf are VS Code forks, so their checks come first
// to avoid false-positive VS Code detection.
func detectFromEnv() (IDEInfo, bool) {
	termProgram := os.Getenv("TERM_PROGRAM")

	// Cursor sets CURSOR_TRACE_ID or TERM_PROGRAM=cursor
	if os.Getenv("CURSOR_TRACE_ID") != "" || termProgram == "cursor" {
		return IDEInfo{
			Type: Cursor,
			Name: "Cursor",
		}, true
	}

	// Windsurf sets WINDSURF_PID or TERM_PROGRAM=windsurf
	if os.Getenv("WINDSURF_PID") != "" || termProgram == "windsurf" {
		return IDEInfo{
			Type: Windsurf,
			Name: "Windsurf",
		}, true
	}

	// Zed sets ZED_TERM or TERM_PROGRAM=zed
	if os.Getenv("ZED_TERM") != "" || termProgram == "zed" {
		return IDEInfo{
			Type: Zed,
			Name: "Zed",
		}, true
	}

	// VS Code sets VSCODE_PID or TERM_PROGRAM=vscode
	if pidStr := os.Getenv("VSCODE_PID"); pidStr != "" {
		pid, _ := strconv.Atoi(pidStr)
		return IDEInfo{
			Type: VSCode,
			Name: "VS Code",
			PID:  pid,
		}, true
	}

	if termProgram == "vscode" {
		return IDEInfo{
			Type: VSCode,
			Name: "VS Code",
		}, true
	}

	// JetBrains sets JETBRAINS_IDE or INTELLIJ_ENVIRONMENT_READER
	if ideName := os.Getenv("JETBRAINS_IDE"); ideName != "" {
		return IDEInfo{
			Type: JetBrains,
			Name: ideName,
		}, true
	}

	if os.Getenv("INTELLIJ_ENVIRONMENT_READER") != "" {
		return IDEInfo{
			Type: JetBrains,
			Name: "JetBrains IDE",
		}, true
	}

	return IDEInfo{}, false
}

// detectFromProcesses scans ancestor processes for known IDE binaries.
// Cursor and Windsurf checks come before VS Code because they are forks.
func detectFromProcesses() IDEInfo {
	ancestors := getAncestorProcesses()

	for _, name := range ancestors {
		base := strings.ToLower(filepath.Base(name))

		// Check Cursor first (VS Code fork)
		for _, bin := range knownCursorBinaries {
			if base == bin {
				return IDEInfo{
					Type: Cursor,
					Name: "Cursor",
				}
			}
		}

		// Check Windsurf (VS Code fork)
		for _, bin := range knownWindsurfBinaries {
			if base == bin {
				return IDEInfo{
					Type: Windsurf,
					Name: "Windsurf",
				}
			}
		}

		// Check Zed
		for _, bin := range knownZedBinaries {
			if base == bin {
				return IDEInfo{
					Type: Zed,
					Name: "Zed",
				}
			}
		}

		for _, bin := range knownVSCodeBinaries {
			if base == bin {
				return IDEInfo{
					Type: VSCode,
					Name: "VS Code",
				}
			}
		}

		for _, bin := range knownJetBrainsBinaries {
			if base == bin {
				return IDEInfo{
					Type: JetBrains,
					Name: bin,
				}
			}
		}
	}

	return IDEInfo{Type: Unknown}
}

// getAncestorProcesses returns the binary names of ancestor processes,
// starting from the parent and walking up to PID 1.
func getAncestorProcesses() []string {
	if runtime.GOOS == "linux" {
		return getAncestorProcessesLinux()
	}
	// macOS and other platforms: use /proc if available, otherwise empty
	return getAncestorProcessesLinux()
}

// getAncestorProcessesLinux reads /proc/PID/cmdline to walk the process tree.
func getAncestorProcessesLinux() []string {
	var result []string
	pid := os.Getppid()

	for pid > 1 {
		cmdline, err := readProcCmdline(pid)
		if err != nil {
			break
		}
		if cmdline != "" {
			result = append(result, filepath.Base(cmdline))
		}

		ppid, err := readProcPPID(pid)
		if err != nil || ppid == pid {
			break
		}
		pid = ppid
	}

	return result
}

// readProcCmdline reads the command name from /proc/PID/cmdline.
func readProcCmdline(pid int) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return "", err
	}
	// cmdline is null-separated; first field is the binary path
	if idx := indexOf(data, 0); idx >= 0 {
		return string(data[:idx]), nil
	}
	return string(data), nil
}

// readProcPPID reads the parent PID from /proc/PID/status.
func readProcPPID(pid int) (int, error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "PPid:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				return strconv.Atoi(fields[1])
			}
		}
	}
	return 0, fmt.Errorf("PPid not found for pid %d", pid)
}

// indexOf returns the index of the first occurrence of b in data, or -1.
func indexOf(data []byte, b byte) int {
	for i, v := range data {
		if v == b {
			return i
		}
	}
	return -1
}

// Entrypoint returns the detected entrypoint context string.
// Values: "cli" (default), "vscode", "jetbrains", "cursor", "windsurf", "zed".
func Entrypoint() string {
	info := DetectIDE()
	switch info.Type {
	case VSCode:
		return "vscode"
	case JetBrains:
		return "jetbrains"
	case Cursor:
		return "cursor"
	case Windsurf:
		return "windsurf"
	case Zed:
		return "zed"
	default:
		return "cli"
	}
}
