package syncignore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

// Load reads and parses a .syncignore file from the given context directory.
// Returns a slice of patterns to ignore.
// If the file doesn't exist, returns an empty slice (no ignore rules).
// Malformed patterns are logged as errors and skipped.
func Load(contextPath string) ([]string, error) {
	syncignorePath := filepath.Join(contextPath, ".syncignore")

	// Check if .syncignore exists
	if _, err := os.Stat(syncignorePath); os.IsNotExist(err) {
		// No .syncignore file - no ignore rules
		return []string{}, nil
	}

	file, err := os.Open(syncignorePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open .syncignore: %w", err)
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip comments
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Validate pattern (basic check - doublestar will validate during matching)
		// Just ensure it's not obviously malformed
		if strings.Contains(line, "***") {
			fmt.Fprintf(os.Stderr, "Warning: Invalid pattern '%s' in .syncignore line %d (skipping)\n", line, lineNum)
			continue
		}

		patterns = append(patterns, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read .syncignore: %w", err)
	}

	// Always add .syncignore itself
	patterns = append(patterns, ".syncignore")

	return patterns, nil
}

// ShouldIgnore checks if a relative file path matches any ignore pattern.
// relativePath should be relative to the context directory.
// Returns true if the file should be ignored, false otherwise.
func ShouldIgnore(relativePath string, patterns []string) bool {
	// Normalize path separators
	relativePath = filepath.ToSlash(relativePath)

	for _, pattern := range patterns {
		// Normalize pattern
		pattern = filepath.ToSlash(pattern)

		// Try matching with doublestar
		matched, err := doublestar.Match(pattern, relativePath)
		if err != nil {
			// Pattern is malformed - log and skip
			fmt.Fprintf(os.Stderr, "Warning: Invalid glob pattern '%s': %v\n", pattern, err)
			continue
		}

		if matched {
			return true
		}

		// Also check if pattern matches with ** prefix for files anywhere
		// e.g., "*.log" should match "foo/bar/test.log"
		if !strings.HasPrefix(pattern, "**/") && !strings.HasPrefix(pattern, "/") {
			globalPattern := "**/" + pattern
			matched, err := doublestar.Match(globalPattern, relativePath)
			if err == nil && matched {
				return true
			}
		}

		// Handle directory patterns (trailing slash)
		if strings.HasSuffix(pattern, "/") {
			// Check if file is inside this directory
			dirPattern := strings.TrimSuffix(pattern, "/")
			if strings.HasPrefix(relativePath, dirPattern+"/") {
				return true
			}
			// Also check with ** prefix
			if !strings.HasPrefix(dirPattern, "**/") && !strings.HasPrefix(dirPattern, "/") {
				globalDirPattern := "**/" + dirPattern
				if strings.HasPrefix(relativePath, globalDirPattern+"/") {
					return true
				}
			}
		}

		// Handle root-only patterns (leading slash)
		if strings.HasPrefix(pattern, "/") {
			rootPattern := strings.TrimPrefix(pattern, "/")
			if relativePath == rootPattern {
				return true
			}
		}
	}

	return false
}

// BuildExcludeFlags generates lftp exclude flags from patterns.
// Returns a slice of flags like ["--exclude", ".git", "--exclude-glob", "*.log"]
func BuildExcludeFlags(patterns []string) []string {
	var flags []string

	for _, pattern := range patterns {
		// Normalize pattern
		pattern = filepath.ToSlash(pattern)

		// If pattern contains wildcards, use --exclude-glob
		if strings.ContainsAny(pattern, "*?[]") {
			flags = append(flags, "--exclude-glob", pattern)
		} else {
			// Otherwise use simple --exclude
			flags = append(flags, "--exclude", pattern)
		}
	}

	return flags
}
