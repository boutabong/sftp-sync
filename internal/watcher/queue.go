package watcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sftp-sync/internal/config"
	"sftp-sync/internal/lftp"
	"sftp-sync/internal/syncignore"
)

// UploadQueue manages sequential file uploads with retry logic
type UploadQueue struct {
	queue      chan *uploadTask
	profiles   map[string]*config.Profile
	profilesMu sync.RWMutex
}

type uploadTask struct {
	profileName string
	filePath    string
}

// NewUploadQueue creates a new upload queue
func NewUploadQueue(profiles map[string]*config.Profile) *UploadQueue {
	return &UploadQueue{
		queue:   make(chan *uploadTask, 100), // Buffer up to 100 pending uploads
		profiles: profiles,
	}
}

// Enqueue adds a file to the upload queue
func (q *UploadQueue) Enqueue(profileName, filePath string) {
	// Warn if queue is getting full (80% capacity)
	queueLen := len(q.queue)
	queueCap := cap(q.queue)
	if queueLen >= int(float64(queueCap)*0.8) {
		fmt.Fprintf(os.Stderr, "Warning: Upload queue is %d%% full (%d/%d)\n",
			(queueLen*100)/queueCap, queueLen, queueCap)
	}

	q.queue <- &uploadTask{
		profileName: profileName,
		filePath:    filePath,
	}
}

// Start starts processing the upload queue
func (q *UploadQueue) Start(onSuccess func(profileName, filePath string), onError func(profileName, filePath string, err error, failCount int)) {
	go func() {
		for task := range q.queue {
			q.processUpload(task, onSuccess, onError)
		}
	}()
}

// processUpload handles uploading a single file with retry logic
func (q *UploadQueue) processUpload(task *uploadTask, onSuccess func(string, string), onError func(string, string, error, int)) {
	// Lock for reading profile
	q.profilesMu.RLock()
	profile, exists := q.profiles[task.profileName]
	q.profilesMu.RUnlock()

	if !exists {
		fmt.Fprintf(os.Stderr, "Error: Profile '%s' not found\n", task.profileName)
		return
	}

	// Get absolute paths
	absFile, err := filepath.Abs(task.filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot resolve file path: %v\n", err)
		return
	}

	absContext, err := filepath.Abs(profile.Context)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Cannot resolve context path: %v\n", err)
		return
	}

	// Calculate relative path
	var relPath string
	if strings.HasPrefix(absFile, absContext+"/") {
		relPath = strings.TrimPrefix(absFile, absContext+"/")
	} else if absFile == absContext {
		relPath = filepath.Base(absFile)
	} else {
		fmt.Fprintf(os.Stderr, "Error: File '%s' not within context '%s'\n", absFile, absContext)
		return
	}

	// Check .syncignore
	patterns, err := syncignore.Load(absContext)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load .syncignore: %v\n", err)
		// Continue anyway
	}

	if syncignore.ShouldIgnore(relPath, patterns) {
		fmt.Fprintf(os.Stderr, "Ignored: %s (matched .syncignore)\n", relPath)
		return
	}

	// Retry logic: 3 attempts with exponential backoff (1s, 2s, 4s)
	maxRetries := 3
	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Attempt upload
		err := lftp.PushFile(profile, absFile)
		if err == nil {
			// Success
			onSuccess(task.profileName, relPath)
			return
		}

		lastErr = err

		// If this isn't the last attempt, wait before retrying
		if attempt < maxRetries-1 {
			fmt.Fprintf(os.Stderr, "Upload failed (attempt %d/%d): %s - %v\n", attempt+1, maxRetries, relPath, err)
			time.Sleep(delays[attempt])
		}
	}

	// All retries failed
	onError(task.profileName, relPath, lastErr, maxRetries)
}

// Stop stops the queue processor
func (q *UploadQueue) Stop() {
	close(q.queue)
}

// LockProfiles locks the profiles map for writing
func (q *UploadQueue) LockProfiles() {
	q.profilesMu.Lock()
}

// UnlockProfiles unlocks the profiles map after writing
func (q *UploadQueue) UnlockProfiles() {
	q.profilesMu.Unlock()
}
