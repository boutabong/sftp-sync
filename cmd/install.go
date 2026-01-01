package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"sftp-sync/internal/deps"
)

// InstallDaemon creates the systemd service file
func InstallDaemon() error {
	fmt.Println("✓ Checking dependencies...")

	// Check systemd
	if !deps.Check("systemctl") {
		return fmt.Errorf("systemd not found. Daemon mode requires systemd.")
	}
	fmt.Println("  ✓ systemd found")

	// Check lftp
	if !deps.Check("lftp") {
		return fmt.Errorf("lftp not installed. Install with: sudo pacman -S lftp")
	}
	fmt.Println("  ✓ lftp found")

	// Check notify-send
	if !deps.Check("notify-send") {
		return fmt.Errorf("notify-send not installed. Install with: sudo pacman -S libnotify")
	}
	fmt.Println("  ✓ notify-send found")

	// Find sftp-sync binary location
	binaryPath, err := exec.LookPath("sftp-sync")
	if err != nil {
		return fmt.Errorf("cannot find sftp-sync in PATH: %w", err)
	}

	// Get absolute path
	absBinaryPath, err := filepath.Abs(binaryPath)
	if err != nil {
		return fmt.Errorf("cannot resolve sftp-sync path: %w", err)
	}

	// Create systemd user directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	systemdUserDir := filepath.Join(homeDir, ".config", "systemd", "user")
	if err := os.MkdirAll(systemdUserDir, 0755); err != nil {
		return fmt.Errorf("cannot create systemd user directory: %w", err)
	}

	// Service file template
	serviceContent := fmt.Sprintf(`[Unit]
Description=SFTP-Sync Auto-Sync Daemon
Documentation=https://github.com/deppess/sftp-sync
After=network-online.target

[Service]
Type=simple
ExecStart=%s daemon
Restart=on-failure
RestartSec=10s

StandardOutput=journal
StandardError=journal
SyslogIdentifier=sftp-sync

[Install]
WantedBy=default.target
`, absBinaryPath)

	// Write service file
	servicePath := filepath.Join(systemdUserDir, "sftp-sync-watch.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd daemon
	cmd := exec.Command("systemctl", "--user", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	fmt.Printf("✓ Created systemd service: %s\n", servicePath)
	fmt.Println("\nTo start the daemon:")
	fmt.Println("  systemctl --user start sftp-sync-watch")
	fmt.Println("\nTo enable auto-start on login:")
	fmt.Println("  systemctl --user enable sftp-sync-watch")
	fmt.Println("\nTo view logs:")
	fmt.Println("  journalctl --user -u sftp-sync-watch -f")

	return nil
}

// UninstallDaemon stops, disables, and removes the systemd service
func UninstallDaemon() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}

	servicePath := filepath.Join(homeDir, ".config", "systemd", "user", "sftp-sync-watch.service")

	// Check if service exists
	if _, err := os.Stat(servicePath); os.IsNotExist(err) {
		return fmt.Errorf("daemon not installed (service file not found)")
	}

	// Stop the service
	cmd := exec.Command("systemctl", "--user", "stop", "sftp-sync-watch")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to stop daemon: %v\n", err)
	} else {
		fmt.Println("✓ Stopped daemon")
	}

	// Disable the service
	cmd = exec.Command("systemctl", "--user", "disable", "sftp-sync-watch")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to disable daemon: %v\n", err)
	} else {
		fmt.Println("✓ Disabled auto-start")
	}

	// Remove service file
	if err := os.Remove(servicePath); err != nil {
		return fmt.Errorf("failed to remove service file: %w", err)
	}
	fmt.Println("✓ Removed service file")

	// Reload systemd daemon
	cmd = exec.Command("systemctl", "--user", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %w", err)
	}

	fmt.Println("\n✓ Daemon uninstalled")
	return nil
}
