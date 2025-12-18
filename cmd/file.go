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

// Push uploads a single file
func Push(profileName, filePath string) error {
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

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		notify.Error("SFTP Error", "Cannot determine current directory")
		return fmt.Errorf("cannot determine current directory")
	}

	// Override profile context with cwd
	profile.Context = cwd

	// Get relative path for display
	relPath := filepath.Base(filePath)
	absFile, err := filepath.Abs(filePath)
	if err == nil {
		if rel, err := filepath.Rel(cwd, absFile); err == nil {
			relPath = rel
		}
	}

	notify.Info("SFTP Sync", fmt.Sprintf("Uploading %s...", relPath))

	// Upload file
	if err := lftp.PushFile(profile, filePath); err != nil {
		notify.Error("SFTP Error", fmt.Sprintf("Failed to upload %s", relPath))
		return err
	}

	notify.Success("File Uploaded", fmt.Sprintf("%s → %s", relPath, profile.Host))
	fmt.Printf("✓ Uploaded: %s\n", relPath)
	return nil
}

// Pull downloads a single file
func Pull(profileName, filePath string) error {
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

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		notify.Error("SFTP Error", "Cannot determine current directory")
		return fmt.Errorf("cannot determine current directory")
	}

	// Override profile context with cwd
	profile.Context = cwd

	// Get relative path for display
	relPath := filepath.Base(filePath)

	notify.Info("SFTP Sync", fmt.Sprintf("Downloading %s...", relPath))

	// Download file
	if err := lftp.PullFile(profile, filePath); err != nil {
		notify.Error("SFTP Error", fmt.Sprintf("Failed to download %s", relPath))
		return err
	}

	notify.Success("File Downloaded", fmt.Sprintf("%s ← %s", relPath, profile.Host))
	fmt.Printf("✓ Downloaded: %s\n", relPath)
	return nil
}

// Current uploads the current file (for editor integration)
func Current(profileName, filePath string) error {
	// This is the same as Push but with different messaging
	return Push(profileName, filePath)
}
