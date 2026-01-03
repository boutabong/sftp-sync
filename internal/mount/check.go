package mount

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"time"

	"sftp-sync/internal/config"
)

// IsReachable checks if the remote server is accessible
func IsReachable(profile *config.Profile) error {
	timeout := 5 * time.Second

	if profile.Protocol == "sftp" {
		// For SFTP, try SSH connection
		return checkSSH(profile, timeout)
	}

	// For FTP, try TCP connection to the port
	return checkTCP(profile, timeout)
}

// checkSSH attempts an SSH connection
func checkSSH(profile *config.Profile, timeout time.Duration) error {
	// Use ssh with batch mode and timeout
	addr := fmt.Sprintf("%s@%s", profile.Username, profile.Host)
	cmd := exec.Command("ssh",
		"-o", "ConnectTimeout=5",
		"-o", "BatchMode=yes",
		"-o", "StrictHostKeyChecking=no",
		"-p", fmt.Sprintf("%d", profile.Port),
		addr,
		"exit",
	)

	// We expect this to fail with authentication error, but connection should work
	err := cmd.Run()

	// If error is about authentication, connection is OK
	// If error is about connection/timeout/host, it's not reachable
	if err != nil {
		// Check if it's a connection error vs auth error
		// For now, we'll use a simpler TCP check
		return checkTCP(profile, timeout)
	}

	return nil
}

// checkTCP attempts a TCP connection to the host:port
func checkTCP(profile *config.Profile, timeout time.Duration) error {
	addr := net.JoinHostPort(profile.Host, strconv.Itoa(profile.Port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return fmt.Errorf("cannot reach %s: %w", addr, err)
	}
	conn.Close()
	return nil
}
