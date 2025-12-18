package mount

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sftp-sync/internal/config"
)

const (
	MountBaseDir = ".mounted"
)

// GetMountPoint returns the mount point path for a profile
func GetMountPoint(profileName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, MountBaseDir, profileName), nil
}

// IsMounted checks if a profile is currently mounted
func IsMounted(profileName string) bool {
	mountPoint, err := GetMountPoint(profileName)
	if err != nil {
		return false
	}

	// Check using mountpoint command
	cmd := exec.Command("mountpoint", "-q", mountPoint)
	err = cmd.Run()
	return err == nil
}

// ListMounted returns a list of currently mounted profiles
func ListMounted() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("cannot determine home directory: %w", err)
	}
	baseDir := filepath.Join(home, MountBaseDir)

	// Check if base directory exists
	if _, err := os.Stat(baseDir); os.IsNotExist(err) {
		return []string{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil, err
	}

	var mounted []string
	for _, entry := range entries {
		if entry.IsDir() {
			profileName := entry.Name()
			if IsMounted(profileName) {
				mounted = append(mounted, profileName)
			}
		}
	}

	return mounted, nil
}

// Mount mounts a remote filesystem based on the protocol
func Mount(profileName string, profile *config.Profile) error {
	mountPoint, err := GetMountPoint(profileName)
	if err != nil {
		return err
	}

	// Check if already mounted
	if IsMounted(profileName) {
		return fmt.Errorf("profile '%s' is already mounted at %s", profileName, mountPoint)
	}

	// Check if remote is reachable
	if err := IsReachable(profile); err != nil {
		return fmt.Errorf("remote unreachable: %w", err)
	}

	// Create mount point directory
	if err = os.MkdirAll(mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point: %w", err)
	}

	// Mount based on protocol
	if profile.Protocol == "sftp" {
		err = mountSSHFS(profile, mountPoint)
	} else {
		err = mountRclone(profile, mountPoint)
	}

	if err != nil {
		// Clean up mount point on failure
		os.RemoveAll(mountPoint)
		return err
	}

	// Verify mount succeeded
	if !IsMounted(profileName) {
		os.RemoveAll(mountPoint)
		return fmt.Errorf("mount verification failed")
	}

	return nil
}

// Unmount unmounts a profile's filesystem
func Unmount(profileName string) error {
	mountPoint, err := GetMountPoint(profileName)
	if err != nil {
		return err
	}

	// Check if mounted
	if !IsMounted(profileName) {
		return fmt.Errorf("profile '%s' is not mounted", profileName)
	}

	// Force unmount using fusermount
	cmd := exec.Command("fusermount", "-uz", mountPoint)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("unmount failed: %w", err)
	}

	// Remove directory and contents
	if err := os.RemoveAll(mountPoint); err != nil {
		return fmt.Errorf("failed to remove mount point: %w", err)
	}

	return nil
}

// UnmountAll unmounts all currently mounted profiles
func UnmountAll() error {
	mounted, err := ListMounted()
	if err != nil {
		return err
	}

	var errors []string
	for _, profileName := range mounted {
		if err := Unmount(profileName); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", profileName, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors unmounting: %s", strings.Join(errors, "; "))
	}

	return nil
}

// getMountsFromProcMounts reads /proc/mounts to find FUSE mounts
func getMountsFromProcMounts() (map[string]string, error) {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	mounts := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 2 {
			device := fields[0]
			mountpoint := fields[1]
			mounts[mountpoint] = device
		}
	}

	return mounts, scanner.Err()
}
