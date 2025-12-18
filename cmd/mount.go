package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sftp-sync/internal/config"
	"sftp-sync/internal/deps"
	"sftp-sync/internal/mount"
	"sftp-sync/internal/notify"
)

// Mount mounts a remote filesystem
func Mount(profileName string, openYazi bool) error {
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

	// Check protocol-specific dependencies
	if profile.Protocol == "sftp" {
		if err := deps.CheckRequired("sshfs", "notify-send"); err != nil {
			notify.Error("Mount Error", err.Error())
			return err
		}
	} else {
		if err := deps.CheckRequired("rclone", "notify-send"); err != nil {
			notify.Error("Mount Error", err.Error())
			return err
		}
	}

	// Check yazi and kitty if needed
	if openYazi {
		if err := deps.CheckRequired("yazi", "kitty"); err != nil {
			notify.Error("Mount Error", err.Error())
			return err
		}
	}

	// Perform mount
	notify.Info("SFTP Mount", fmt.Sprintf("Mounting %s...", profileName))

	if err := mount.Mount(profileName, profile); err != nil {
		// Detailed error notification
		errorMsg := err.Error()
		if strings.Contains(errorMsg, "already mounted") {
			mountPoint, _ := mount.GetMountPoint(profileName, profile)
			notify.Error("Mount Error", fmt.Sprintf("Profile '%s' is already mounted at %s", profileName, mountPoint))
		} else if strings.Contains(errorMsg, "unreachable") {
			notify.Error("Mount Error", fmt.Sprintf("Cannot reach %s:%d\n%s", profile.Host, profile.Port, errorMsg))
		} else if strings.Contains(errorMsg, "authentication") {
			notify.Error("Mount Error", fmt.Sprintf("Authentication failed for %s@%s", profile.Username, profile.Host))
		} else {
			notify.Error("Mount Error", fmt.Sprintf("Failed to mount %s\n%s", profileName, errorMsg))
		}
		return err
	}

	mountPoint, err := mount.GetMountPoint(profileName, profile)
	if err != nil {
		notify.Error("Mount Error", err.Error())
		return err
	}

	if openYazi {
		// Launch kitty with yazi
		notify.Success("SFTP Mount", fmt.Sprintf("Mounted %s at %s\nOpening yazi...", profileName, mountPoint))
		fmt.Printf("✓ Mounted %s at %s\n", profileName, mountPoint)
		fmt.Println("Opening yazi...")

		// Launch kitty with custom title and yazi
		title := fmt.Sprintf("SFTP-Mount-%s", profileName)
		cmd := exec.Command("kitty", "-T", title, "-e", "yazi", mountPoint)

		// Run and wait for kitty to exit
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: yazi exited with error: %v\n", err)
		}

		// Auto-unmount when yazi exits
		fmt.Println("Yazi closed, unmounting...")
		if err := mount.Unmount(profileName, profile); err != nil {
			notify.Error("Unmount Error", err.Error())
			return err
		}

		notify.Success("SFTP Unmount", fmt.Sprintf("Unmounted %s", profileName))
		fmt.Printf("✓ Unmounted %s\n", profileName)
	} else {
		// Just mount and print path
		notify.Success("SFTP Mount", fmt.Sprintf("Mounted %s at %s", profileName, mountPoint))
		fmt.Printf("✓ Mounted %s at:\n%s\n", profileName, mountPoint)
	}

	return nil
}

// Unmount unmounts a profile's filesystem
func Unmount(profileName string, unmountAll bool) error {
	if unmountAll {
		// Unmount all profiles
		mounted, err := mount.ListMounted()
		if err != nil {
			notify.Error("Unmount Error", err.Error())
			return err
		}

		if len(mounted) == 0 {
			fmt.Println("No mounted profiles")
			return nil
		}

		fmt.Printf("Unmounting %d profile(s)...\n", len(mounted))
		if err := mount.UnmountAll(); err != nil {
			notify.Error("Unmount Error", err.Error())
			return err
		}

		notify.Success("SFTP Unmount", fmt.Sprintf("Unmounted %d profile(s)", len(mounted)))
		fmt.Printf("✓ Unmounted %d profile(s)\n", len(mounted))
		return nil
	}

	// Load config to get profile (needed for custom context paths)
	cfg, err := config.Load()
	if err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	profile, err := cfg.GetProfile(profileName)
	if err != nil {
		notify.Error("SFTP Sync Error", err.Error())
		return err
	}

	// Unmount single profile
	if err := mount.Unmount(profileName, profile); err != nil {
		notify.Error("Unmount Error", err.Error())
		return err
	}

	notify.Success("SFTP Unmount", fmt.Sprintf("Unmounted %s", profileName))
	fmt.Printf("✓ Unmounted %s\n", profileName)
	return nil
}

// Mounts lists all currently mounted profiles
func Mounts() error {
	mounted, err := mount.ListMounted()
	if err != nil {
		return err
	}

	if len(mounted) == 0 {
		fmt.Println("No mounted profiles")
		return nil
	}

	fmt.Printf("Currently mounted profiles (%d):\n", len(mounted))
	for _, profileName := range mounted {
		// Use nil profile to get default mount point
		mountPoint, err := mount.GetMountPoint(profileName, nil)
		if err != nil {
			fmt.Printf("  • %s → (error: %v)\n", profileName, err)
			continue
		}
		fmt.Printf("  • %s → %s\n", profileName, mountPoint)
	}

	return nil
}
