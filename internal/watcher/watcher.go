package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"

	"sftp-sync/internal/config"
)

// Watcher watches file changes for auto-sync
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	debouncer *Debouncer
	profiles  map[string]*config.Profile // profile name -> profile
	callbacks map[string]func(string)    // profile name -> upload callback
}

// New creates a new watcher
func New() (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file watcher: %w", err)
	}

	return &Watcher{
		fsWatcher: fsWatcher,
		debouncer: NewDebouncer(),
		profiles:  make(map[string]*config.Profile),
		callbacks: make(map[string]func(string)),
	}, nil
}

// Watch starts watching a profile's context directory
func (w *Watcher) Watch(profileName string, profile *config.Profile, callback func(filePath string)) error {
	// Validate context exists
	if _, err := os.Stat(profile.Context); os.IsNotExist(err) {
		return fmt.Errorf("context directory doesn't exist: %s", profile.Context)
	}

	// Store profile and callback
	w.profiles[profileName] = profile
	w.callbacks[profileName] = callback

	// Add context directory to watcher (recursively)
	if err := w.addRecursive(profile.Context); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Watching: %s (%s)\n", profileName, profile.Context)
	return nil
}

// addRecursive adds a directory and all its subdirectories to the watcher
func (w *Watcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		// Only watch directories
		if info.IsDir() {
			if err := w.fsWatcher.Add(path); err != nil {
				return err
			}
		}

		return nil
	})
}

// Unwatch stops watching a profile
func (w *Watcher) Unwatch(profileName string) error {
	profile, exists := w.profiles[profileName]
	if !exists {
		return fmt.Errorf("profile not watched: %s", profileName)
	}

	// Remove context directory from watcher
	if err := w.removeRecursive(profile.Context); err != nil {
		return err
	}

	// Remove from maps
	delete(w.profiles, profileName)
	delete(w.callbacks, profileName)

	fmt.Fprintf(os.Stderr, "Stopped watching: %s\n", profileName)
	return nil
}

// removeRecursive removes a directory and all subdirectories from watcher
func (w *Watcher) removeRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue even if file doesn't exist
		}

		if info.IsDir() {
			w.fsWatcher.Remove(path)
		}

		return nil
	})
}

// Start starts processing file system events
func (w *Watcher) Start() {
	go func() {
		for {
			select {
			case event, ok := <-w.fsWatcher.Events:
				if !ok {
					return
				}

				// Only process WRITE and CREATE events
				if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
					w.handleEvent(event)
				}

			case err, ok := <-w.fsWatcher.Errors:
				if !ok {
					return
				}
				fmt.Fprintf(os.Stderr, "Watcher error: %v\n", err)
			}
		}
	}()
}

// handleEvent processes a file system event
func (w *Watcher) handleEvent(event fsnotify.Event) {
	filePath := event.Name

	// Check if file is a directory or symlink (ignore)
	info, err := os.Lstat(filePath)
	if err != nil {
		// File might have been deleted, ignore
		return
	}

	if info.IsDir() {
		// Directory created - add to watcher
		if event.Op&fsnotify.Create == fsnotify.Create {
			w.fsWatcher.Add(filePath)
		}
		return
	}

	// Skip symlinks
	if info.Mode()&os.ModeSymlink != 0 {
		return
	}

	// Find which profile(s) this file belongs to
	matchedProfiles := w.findMatchingProfiles(filePath)

	for _, profileName := range matchedProfiles {
		profile := w.profiles[profileName]
		callback := w.callbacks[profileName]

		// Get debounce delay
		delay := time.Duration(profile.AutoSyncDebounce) * time.Millisecond
		if delay == 0 {
			delay = 2000 * time.Millisecond // Default 2s
		}

		// Debounce key: profileName + filePath
		debounceKey := profileName + ":" + filePath

		// Add debounced callback
		w.debouncer.Add(debounceKey, delay, func() {
			callback(filePath)
		})
	}
}

// findMatchingProfiles finds which profile(s) a file belongs to
// Returns profile names sorted by context specificity (most specific first)
func (w *Watcher) findMatchingProfiles(filePath string) []string {
	var matches []struct {
		name  string
		depth int
	}

	for name, profile := range w.profiles {
		// Check if file is under this profile's context
		if strings.HasPrefix(filePath, profile.Context+"/") || filePath == profile.Context {
			// Calculate depth (number of path separators)
			depth := strings.Count(profile.Context, string(os.PathSeparator))
			matches = append(matches, struct {
				name  string
				depth int
			}{name, depth})
		}
	}

	if len(matches) == 0 {
		return nil
	}

	// Sort by depth (most specific = deepest path = highest depth)
	// For overlapping contexts, we want most specific only
	maxDepth := -1
	for _, m := range matches {
		if m.depth > maxDepth {
			maxDepth = m.depth
		}
	}

	// Return only the most specific matches
	var result []string
	for _, m := range matches {
		if m.depth == maxDepth {
			result = append(result, m.name)
		}
	}

	return result
}

// Close stops the watcher and cleans up
func (w *Watcher) Close() error {
	w.debouncer.StopAll()
	return w.fsWatcher.Close()
}
