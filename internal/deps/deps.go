package deps

import (
	"fmt"
	"os/exec"
	"strings"
)

// Dependency represents an external command
type Dependency struct {
	Name        string
	Description string
}

// Core dependencies required for basic operations
var CoreDeps = []Dependency{
	{"lftp", "FTP/SFTP client (sync operations)"},
	{"notify-send", "Desktop notifications"},
}

// Protocol-specific dependencies
var ProtocolDeps = []Dependency{
	{"sshfs", "SFTP mounting"},
	{"rclone", "FTP mounting"},
}

// Optional dependencies for enhanced features
var OptionalDeps = []Dependency{
	{"kitty", "Terminal emulator (required for --yazi)"},
	{"yazi", "File manager (optional, for --yazi flag)"},
}

// Check verifies if a command is available in PATH
func Check(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// CheckRequired verifies only the dependencies needed for a specific operation
func CheckRequired(deps ...string) error {
	var missing []string
	for _, dep := range deps {
		if !Check(dep) {
			missing = append(missing, dep)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing dependencies: %s", strings.Join(missing, ", "))
	}

	return nil
}
