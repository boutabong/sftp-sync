package lftp

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"sftp-sync/internal/config"
)

// Result represents the outcome of an lftp operation
type Result struct {
	Success      bool
	FileCount    int
	Output       string
	Error        error
	HasFtpQuota  bool
	ErrorMessage string
}

// buildConnection builds the lftp connection string
func buildConnection(profile *config.Profile) string {
	return fmt.Sprintf("%s://%s", profile.Protocol, profile.Host)
}

// buildCommand builds the lftp command with common settings
func buildCommand(profile *config.Profile, ftpCommand string) *exec.Cmd {
	connection := buildConnection(profile)

	// Build settings string
	var settings string

	// Check if using SSH key for SFTP
	if profile.Protocol == "sftp" && profile.SSHKey != "" {
		// Use SSH key authentication
		sshCmd := fmt.Sprintf("ssh -a -x -i %s", profile.SSHKey)
		settings = fmt.Sprintf("set sftp:connect-program '%s'; set ftp:ssl-allow no; set ssl:verify-certificate no; %s", sshCmd, ftpCommand)

		// For SSH key auth, use empty password to prevent password prompt
		credentials := profile.Username + ","
		args := []string{
			"-e", settings + "; quit",
			"-u", credentials,
			"-p", fmt.Sprintf("%d", profile.Port),
			connection,
		}
		return exec.Command("lftp", args...)
	}

	// Default: password authentication
	credentials := fmt.Sprintf("%s,%s", profile.Username, profile.Password)
	settings = fmt.Sprintf("set ftp:ssl-allow no; set ssl:verify-certificate no; %s", ftpCommand)

	args := []string{
		"-e", settings + "; quit",
		"-u", credentials,
		"-p", fmt.Sprintf("%d", profile.Port),
		connection,
	}

	return exec.Command("lftp", args...)
}

// SyncUp uploads local directory to remote (mirror -R)
func SyncUp(profile *config.Profile) (*Result, error) {
	// Verify local path exists
	absLocal, err := filepath.Abs(profile.Context)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve local path: %w", err)
	}

	ftpCmd := fmt.Sprintf("mirror -R --verbose --delete '%s' '%s'", absLocal, profile.RemotePath)
	cmd := buildCommand(profile, ftpCmd)

	output, err := cmd.CombinedOutput()
	return parseResult(output, err)
}

// SyncDown downloads remote directory to local (mirror)
func SyncDown(profile *config.Profile) (*Result, error) {
	// Verify local path exists
	absLocal, err := filepath.Abs(profile.Context)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve local path: %w", err)
	}

	ftpCmd := fmt.Sprintf("mirror --verbose --delete '%s' '%s'", profile.RemotePath, absLocal)
	cmd := buildCommand(profile, ftpCmd)

	output, err := cmd.CombinedOutput()
	return parseResult(output, err)
}

// Diff shows what would be uploaded (dry-run)
func Diff(profile *config.Profile) error {
	ftpCmd := fmt.Sprintf("mirror -R --dry-run --verbose '%s' '%s'", profile.Context, profile.RemotePath)
	cmd := buildCommand(profile, ftpCmd)

	cmd.Stdout = nil // Output goes directly to terminal
	cmd.Stderr = nil

	return cmd.Run()
}

// PushFile uploads a single file
func PushFile(profile *config.Profile, filePath string) error {
	// Calculate relative path from local context
	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("cannot resolve file path: %w", err)
	}

	absLocal, err := filepath.Abs(profile.Context)
	if err != nil {
		return fmt.Errorf("cannot resolve local path: %w", err)
	}

	// Check if file is within context
	if !strings.HasPrefix(absFile, absLocal+"/") && absFile != absLocal {
		return fmt.Errorf("file '%s' is not within context '%s'", absFile, absLocal)
	}

	// Calculate relative path and remote file location
	relPath := strings.TrimPrefix(absFile, absLocal+"/")
	remoteFile := filepath.Join(profile.RemotePath, relPath)
	remoteDir := filepath.Dir(remoteFile)

	ftpCmd := fmt.Sprintf("put -O '%s' '%s'", remoteDir, absFile)
	cmd := buildCommand(profile, ftpCmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("upload failed: %s", parseError(string(output)))
	}

	return nil
}

// PullFile downloads a single file
func PullFile(profile *config.Profile, filePath string) error {
	// Build absolute file path
	var absFile string
	if filepath.IsAbs(filePath) {
		absFile = filePath
	} else {
		absLocal, err := filepath.Abs(profile.Context)
		if err != nil {
			return fmt.Errorf("cannot resolve local path: %w", err)
		}
		absFile = filepath.Join(absLocal, filePath)
	}

	absLocal, err := filepath.Abs(profile.Context)
	if err != nil {
		return fmt.Errorf("cannot resolve local path: %w", err)
	}

	// Check if file is within context
	if !strings.HasPrefix(absFile, absLocal+"/") && absFile != absLocal {
		return fmt.Errorf("file '%s' is not within context '%s'", absFile, absLocal)
	}

	// Calculate relative path and remote file location
	relPath := strings.TrimPrefix(absFile, absLocal+"/")
	remoteFile := filepath.Join(profile.RemotePath, relPath)

	ftpCmd := fmt.Sprintf("get '%s' -o '%s'", remoteFile, absFile)
	cmd := buildCommand(profile, ftpCmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("download failed: %s", parseError(string(output)))
	}

	return nil
}

// parseResult parses lftp output and determines success/failure
func parseResult(output []byte, err error) (*Result, error) {
	result := &Result{
		Output: string(output),
	}

	// Check for .ftpquota errors
	hasFtpQuota := strings.Contains(result.Output, ".ftpquota")
	result.HasFtpQuota = hasFtpQuota

	// Count transferred/removed files
	transferPattern := regexp.MustCompile(`(?i)(Transferring|Removing)`)
	matches := transferPattern.FindAllString(result.Output, -1)
	result.FileCount = len(matches)

	// Check for errors (excluding .ftpquota)
	errorPattern := regexp.MustCompile(`(?i)(error|failed|prohibited)`)
	errorLines := errorPattern.FindAllString(result.Output, -1)
	nonFtpQuotaErrors := 0
	for _, line := range errorLines {
		if !strings.Contains(line, ".ftpquota") {
			nonFtpQuotaErrors++
		}
	}

	// Determine success
	if err != nil && hasFtpQuota && nonFtpQuotaErrors == 0 {
		// Only .ftpquota errors - treat as warning
		result.Success = true
		result.ErrorMessage = "Warning: .ftpquota is server-protected"
	} else if err != nil {
		// Real errors
		result.Success = false
		result.Error = err
		result.ErrorMessage = parseError(result.Output)
	} else {
		// Success
		result.Success = true
	}

	return result, nil
}

// parseError extracts meaningful error messages from lftp output
func parseError(output string) string {
	if strings.Contains(output, "Connection refused") {
		return "Connection refused"
	}
	if strings.Contains(output, "Login incorrect") {
		return "Authentication failed"
	}
	if strings.Contains(output, "Permission denied") {
		return "Permission denied"
	}
	if strings.Contains(output, "Name or service not known") {
		return "Host not found"
	}
	if strings.Contains(output, "No such file or directory") {
		return "File or directory not found"
	}

	// Return first non-empty line as error
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}

	return "Unknown error"
}
