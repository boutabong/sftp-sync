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

	// Build profiles map for queue
	profiles := make(map[string]*config.Profile)
	for name, profile := range cfg.Profiles {
		p := profile
		profiles[name] = &p
	}

	// Create upload queue
	queue := watcher.NewUploadQueue(profiles)

	// Create notifier with batching
	notifier := watcher.NewNotifier()

	// Start queue processor
	queue.Start(
		// On success
		func(profileName, relPath string) {
			fmt.Fprintf(os.Stderr, "✓ Uploaded: %s → %s\n", relPath, profileName)
			notifier.ResetErrorCount(profileName)
			notifier.NotifySuccess(profileName, relPath)
		},
		// On error
		func(profileName, relPath string, err error, failCount int) {
			fmt.Fprintf(os.Stderr, "✗ Upload failed after %d attempts: %s → %s (%v)\n", failCount, relPath, profileName, err)
			notifier.NotifyError(profileName, relPath, err)
		},
	)

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
			// Enqueue upload
			queue.Enqueue(name, filePath)
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
	queue.Stop()
	return nil
}
