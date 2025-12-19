package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"sftp-sync/internal/config"
	"sftp-sync/internal/deps"
	"sftp-sync/internal/lftp"
	"sftp-sync/internal/notify"
)

// getContext determines the context directory
// If contextFile is provided and absolute, finds project root
// Otherwise uses cwd
func getContext(contextFile string) (string, error) {
	if contextFile == "" {
		// No file provided, use cwd
		return os.Getwd()
	}

	// File provided - use smart detection (same logic as findProjectRoot)
	if !filepath.IsAbs(contextFile) {
		// Relative path, use cwd
		return os.Getwd()
	}

	// Absolute path - find project root
	dir := filepath.Dir(contextFile)
	homeDir, _ := os.UserHomeDir()

	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir, nil
		}

		if dir == homeDir || dir == "/" {
			return filepath.Dir(contextFile), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return filepath.Dir(contextFile), nil
		}
		dir = parent
	}
}

// Up performs full upload sync
func Up(profileName, contextFile string) error {
	// Check dependencies
	if err := deps.CheckRequired("lftp", "notify-send"); err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Get profile
	profile, err := cfg.GetProfile(profileName)
	if err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Get context directory (smart detection for editor integration)
	contextDir, err := getContext(contextFile)
	if err != nil {
		msg := "Cannot determine context directory"
		notify.Error("SFTP Error", msg)
		return fmt.Errorf(msg)
	}

	// Override profile context with detected context
	profile.Context = contextDir

	fmt.Fprintf(os.Stderr, "Debug: Uploading from '%s' to '%s' on %s\n", contextDir, profile.RemotePath, profile.Host)
	notify.Info("SFTP Sync", fmt.Sprintf("Uploading to %s...", profile.Host))

	// Perform sync
	result, err := lftp.SyncUp(profile)
	if err != nil {
		notify.Error("SFTP Error", err.Error())
		return err
	}

	// Handle result
	if result.Success {
		if result.HasFtpQuota {
			msg := fmt.Sprintf("Uploaded to %s\nFiles synced: %d\n(Warning: .ftpquota protected)", profile.Host, result.FileCount)
			notify.Warning("SFTP Sync Complete", msg)
			fmt.Printf("⚠ Upload complete: %d files synced (Warning: .ftpquota is server-protected)\n", result.FileCount)
		} else {
			msg := fmt.Sprintf("Uploaded to %s\nFiles synced: %d", profile.Host, result.FileCount)
			notify.Success("SFTP Sync Complete", msg)
			fmt.Printf("✓ Upload complete: %d files synced\n", result.FileCount)
		}
		return nil
	}

	// Handle errors
	notify.Error("SFTP Error", fmt.Sprintf("Upload failed: %s", result.ErrorMessage))
	fmt.Fprintf(os.Stderr, "✗ Upload failed!\n")
	fmt.Fprintf(os.Stderr, "✗ Error: %s\n", result.ErrorMessage)
	return fmt.Errorf("upload failed: %s", result.ErrorMessage)
}

// Down performs full download sync
func Down(profileName, contextFile string) error {
	// Check dependencies
	if err := deps.CheckRequired("lftp", "notify-send"); err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Get profile
	profile, err := cfg.GetProfile(profileName)
	if err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Get context directory (smart detection for editor integration)
	contextDir, err := getContext(contextFile)
	if err != nil {
		msg := "Cannot determine context directory"
		notify.Error("SFTP Error", msg)
		return fmt.Errorf(msg)
	}

	// Override profile context with detected context
	profile.Context = contextDir

	fmt.Fprintf(os.Stderr, "Debug: Downloading from '%s' on %s to '%s'\n", profile.RemotePath, profile.Host, contextDir)
	notify.Info("SFTP Sync", fmt.Sprintf("Downloading from %s...", profile.Host))

	// Perform sync
	result, err := lftp.SyncDown(profile)
	if err != nil {
		notify.Error("SFTP Error", err.Error())
		return err
	}

	// Handle result
	if result.Success {
		if result.HasFtpQuota {
			msg := fmt.Sprintf("Downloaded from %s\nFiles synced: %d\n(Warning: .ftpquota protected)", profile.Host, result.FileCount)
			notify.Warning("SFTP Sync Complete", msg)
			fmt.Printf("⚠ Download complete: %d files synced (Warning: .ftpquota is server-protected)\n", result.FileCount)
		} else {
			msg := fmt.Sprintf("Downloaded from %s\nFiles synced: %d", profile.Host, result.FileCount)
			notify.Success("SFTP Sync Complete", msg)
			fmt.Printf("✓ Download complete: %d files synced\n", result.FileCount)
		}
		return nil
	}

	// Handle errors
	notify.Error("SFTP Error", fmt.Sprintf("Download failed: %s", result.ErrorMessage))
	fmt.Fprintf(os.Stderr, "✗ Download failed!\n")
	fmt.Fprintf(os.Stderr, "✗ Error: %s\n", result.ErrorMessage)
	return fmt.Errorf("download failed: %s", result.ErrorMessage)
}

// Diff shows what would be uploaded (dry-run)
func Diff(profileName, contextFile string) error {
	// Check dependencies
	if err := deps.CheckRequired("lftp", "notify-send"); err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Get profile
	profile, err := cfg.GetProfile(profileName)
	if err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Get context directory (smart detection for editor integration)
	contextDir, err := getContext(contextFile)
	if err != nil {
		notify.Error("SFTP Error", "Cannot determine context directory")
		return fmt.Errorf("cannot determine context directory")
	}

	// Override profile context with detected context
	profile.Context = contextDir

	notify.Info("SFTP Sync", "Comparing local vs remote...")

	if err := lftp.Diff(profile); err != nil {
		notify.Error("SFTP Error", "Diff failed")
		return err
	}

	notify.Success("SFTP Diff Complete", "Check terminal for differences")
	return nil
}
