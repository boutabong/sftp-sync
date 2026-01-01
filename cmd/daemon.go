package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sftp-sync/internal/config"
	"sftp-sync/internal/deps"
	"sftp-sync/internal/watcher"
)

// Daemon runs the auto-sync daemon
func Daemon() error {
	// Check dependencies
	if err := deps.CheckRequired("lftp", "notify-send"); err != nil {
		return err
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create watcher
	w, err := watcher.New()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer w.Close()

	// Find and watch all profiles with autoSync enabled
	watchedCount := 0
	for name, profile := range cfg.Profiles {
		// Make a copy of the profile to avoid pointer issues
		p := profile

		if !p.AutoSync {
			continue
		}

		// Validate context exists
		if p.Context == "" {
			fmt.Fprintf(os.Stderr, "Warning: Profile '%s' has autoSync enabled but no context set (skipping)\n", name)
			continue
		}

		// Watch this profile
		err := w.Watch(name, &p, func(filePath string) {
			fmt.Fprintf(os.Stderr, "Change detected: %s (profile: %s)\n", filePath, name)
			// TODO: Upload will be implemented in Phase 3
		})

		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to watch profile '%s': %v (skipping)\n", name, err)
			continue
		}

		watchedCount++
	}

	if watchedCount == 0 {
		return fmt.Errorf("no profiles with autoSync enabled found")
	}

	fmt.Fprintf(os.Stderr, "Daemon started, watching %d profile(s)\n", watchedCount)

	// Start processing events
	w.Start()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Fprintf(os.Stderr, "\nDaemon stopping...\n")
	return nil
}
