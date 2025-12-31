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

// findProjectRoot determines the appropriate context directory for a file operation
// Priority: 1) Config context (if set), 2) Detect from .git, 3) Current working directory
func findProjectRoot(profile *config.Profile, filePath string) (string, error) {
	// If context is explicitly set in config, use it
	if profile.Context != "" {
		return profile.Context, nil
	}

	// No context in config - try to detect it
	// If path is relative, use cwd
	if !filepath.IsAbs(filePath) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot determine current directory: %w", err)
		}
		return cwd, nil
	}

	// For absolute paths (editor scenario), find project root
	dir := filepath.Dir(filePath)
	homeDir, _ := os.UserHomeDir()

	// Walk up the directory tree looking for .git
	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			// Found .git directory - this is the project root
			return dir, nil
		}

		// Stop at home directory or root
		if dir == homeDir || dir == "/" {
			// No .git found - return error instead of guessing
			return "", fmt.Errorf("no project root found (no .git directory). Please set 'context' in config for profile")
		}

		// Move up one directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", fmt.Errorf("no project root found (filesystem root reached). Please set 'context' in config for profile")
		}
		dir = parent
	}
}

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

	// Smart context detection: respects config, falls back to .git detection
	contextDir, err := findProjectRoot(profile, filePath)
	if err != nil {
		notify.Error("SFTP Error", err.Error())
		return err
	}

	// Set the resolved context (only if it wasn't already set from config)
	if profile.Context == "" {
		profile.Context = contextDir
	}

	// Get relative path for display
	relPath := filepath.Base(filePath)
	absFile, err := filepath.Abs(filePath)
	if err == nil {
		if rel, err := filepath.Rel(contextDir, absFile); err == nil {
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

	// Smart context detection: respects config, falls back to .git detection
	contextDir, err := findProjectRoot(profile, filePath)
	if err != nil {
		notify.Error("SFTP Error", err.Error())
		return err
	}

	// Set the resolved context (only if it wasn't already set from config)
	if profile.Context == "" {
		profile.Context = contextDir
	}

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
