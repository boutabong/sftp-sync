package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/fsnotify/fsnotify"

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

	// Watch config file for changes
	configWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to create config watcher: %v\n", err)
	} else {
		defer configWatcher.Close()

		// Get config file path
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".config", "sftp-sync", "config.json")

		err = configWatcher.Add(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to watch config file: %v\n", err)
		} else {
			// Handle config changes in background
			go func() {
				for {
					select {
					case event, ok := <-configWatcher.Events:
						if !ok {
							return
						}

						// Config file changed
						if event.Op&fsnotify.Write == fsnotify.Write {
							fmt.Fprintf(os.Stderr, "Config file changed, reloading...\n")
							handleConfigReload(w, profiles, queue)
						}

					case err, ok := <-configWatcher.Errors:
						if !ok {
							return
						}
						fmt.Fprintf(os.Stderr, "Config watcher error: %v\n", err)
					}
				}
			}()
		}
	}

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Fprintf(os.Stderr, "\nDaemon stopping...\n")
	queue.Stop()
	return nil
}

// handleConfigReload reloads config and adjusts watched profiles
func handleConfigReload(w *watcher.Watcher, profiles map[string]*config.Profile, queue *watcher.UploadQueue) {
	// Load new config
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
		return
	}

	// Build new profiles map
	newProfiles := make(map[string]*config.Profile)
	for name, profile := range cfg.Profiles {
		p := profile
		newProfiles[name] = &p
	}

	// Compare old vs new profiles
	// 1. Stop watching profiles that were removed or have autoSync disabled
	for oldName, oldProfile := range profiles {
		newProfile, exists := newProfiles[oldName]

		if !exists || !newProfile.AutoSync {
			// Profile removed or autoSync disabled
			err := w.Unwatch(oldName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to unwatch profile '%s': %v\n", oldName, err)
			} else {
				fmt.Fprintf(os.Stderr, "Stopped watching: %s (removed or autoSync disabled)\n", oldName)
			}
			delete(profiles, oldName)
		} else if newProfile.Context != oldProfile.Context {
			// Context changed - restart watching
			err := w.Unwatch(oldName)
			if err == nil {
				err = w.Watch(oldName, newProfile, func(filePath string) {
					queue.Enqueue(oldName, filePath)
				})
				if err != nil {
					fmt.Fprintf(os.Stderr, "Warning: Failed to restart watching '%s': %v\n", oldName, err)
				} else {
					fmt.Fprintf(os.Stderr, "Restarted watching: %s (context changed)\n", oldName)
					profiles[oldName] = newProfile
				}
			}
		}
	}

	// 2. Start watching new profiles with autoSync enabled
	for newName, newProfile := range newProfiles {
		if !newProfile.AutoSync {
			continue
		}

		_, alreadyWatching := profiles[newName]
		if !alreadyWatching {
			// New profile with autoSync
			if newProfile.Context == "" {
				fmt.Fprintf(os.Stderr, "Warning: Profile '%s' has autoSync enabled but no context set (skipping)\n", newName)
				continue
			}

			err := w.Watch(newName, newProfile, func(filePath string) {
				queue.Enqueue(newName, filePath)
			})

			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to watch new profile '%s': %v\n", newName, err)
			} else {
				fmt.Fprintf(os.Stderr, "Started watching: %s (autoSync enabled)\n", newName)
				profiles[newName] = newProfile
			}
		}
	}
}
