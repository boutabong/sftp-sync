package main

import (
	"fmt"
	"os"
	"runtime"

	"sftp-sync/cmd"
)

const version = "2.2.0"

func main() {
	// Linux-only check
	if runtime.GOOS != "linux" {
		fmt.Fprintf(os.Stderr, "Error: sftp-sync is Linux-only\n")
		fmt.Fprintf(os.Stderr, "Current OS: %s\n", runtime.GOOS)
		fmt.Fprintf(os.Stderr, "\nThis tool requires Linux-specific features:\n")
		fmt.Fprintf(os.Stderr, "  - FUSE filesystem support (sshfs/rclone)\n")
		fmt.Fprintf(os.Stderr, "  - notify-send (libnotify)\n")
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "up":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sftp-sync up <profile> [file]")
			os.Exit(1)
		}
		// Optional file argument for editor integration
		var contextFile string
		if len(os.Args) >= 4 {
			contextFile = os.Args[3]
		}
		if err := cmd.Up(os.Args[2], contextFile); err != nil {
			os.Exit(1)
		}

	case "down":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sftp-sync down <profile> [file]")
			os.Exit(1)
		}
		// Optional file argument for editor integration
		var contextFile string
		if len(os.Args) >= 4 {
			contextFile = os.Args[3]
		}
		if err := cmd.Down(os.Args[2], contextFile); err != nil {
			os.Exit(1)
		}

	case "diff":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sftp-sync diff <profile> [file]")
			os.Exit(1)
		}
		// Optional file argument for editor integration
		var contextFile string
		if len(os.Args) >= 4 {
			contextFile = os.Args[3]
		}
		if err := cmd.Diff(os.Args[2], contextFile); err != nil {
			os.Exit(1)
		}

	case "push":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sftp-sync push <profile> <file>")
			os.Exit(1)
		}
		if err := cmd.Push(os.Args[2], os.Args[3]); err != nil {
			os.Exit(1)
		}

	case "pull":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sftp-sync pull <profile> <file>")
			os.Exit(1)
		}
		if err := cmd.Pull(os.Args[2], os.Args[3]); err != nil {
			os.Exit(1)
		}

	case "current":
		if len(os.Args) < 4 {
			fmt.Println("Usage: sftp-sync current <profile> <file>")
			os.Exit(1)
		}
		if err := cmd.Current(os.Args[2], os.Args[3]); err != nil {
			os.Exit(1)
		}

	case "mount":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sftp-sync mount <profile> [--yazi]")
			os.Exit(1)
		}
		profileName := os.Args[2]
		openYazi := false
		if len(os.Args) >= 4 && os.Args[3] == "--yazi" {
			openYazi = true
		}
		if err := cmd.Mount(profileName, openYazi); err != nil {
			os.Exit(1)
		}

	case "unmount":
		if len(os.Args) < 3 {
			fmt.Println("Usage: sftp-sync unmount <profile|--all>")
			os.Exit(1)
		}
		if os.Args[2] == "--all" {
			if err := cmd.Unmount("", true); err != nil {
				os.Exit(1)
			}
		} else {
			if err := cmd.Unmount(os.Args[2], false); err != nil {
				os.Exit(1)
			}
		}

	case "mounts":
		if err := cmd.Mounts(); err != nil {
			os.Exit(1)
		}

	case "version", "--version", "-v":
		fmt.Printf("sftp-sync version %s\n", version)

	case "help", "--help", "-h":
		printUsage()

	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`sftp-sync - FTP/SFTP synchronization and mounting tool

USAGE:
  sftp-sync <command> <profile> [options]

SYNC COMMANDS:
  up <profile>              Upload local directory to remote (full sync)
  down <profile>            Download remote directory to local (full sync)
  diff <profile>            Show what would be uploaded (dry-run)
  push <profile> <file>     Upload a single file
  pull <profile> <file>     Download a single file
  current <profile> <file>  Upload current file (editor integration)

MOUNT COMMANDS:
  mount <profile>           Mount remote filesystem
  mount <profile> --yazi    Mount and open in yazi file manager
  unmount <profile>         Unmount a profile's filesystem
  unmount --all             Unmount all mounted filesystems
  mounts                    List currently mounted profiles

OTHER:
  version                   Show version information
  help                      Show this help message

CONFIGURATION:
  Config file: ~/.config/sftp-sync/config.json

  Example config:
  {
    "myserver": {
      "host": "ftp.example.com",
      "username": "user",
      "password": "pass",
      "port": 21,
      "protocol": "ftp",
      "remotePath": "/public_html",
      "context": "/home/user/projects/website"
    }
  }

EXAMPLES:
  sftp-sync up myserver
  sftp-sync mount myserver --yazi
  sftp-sync push myserver index.html
  sftp-sync unmount --all
`)
}
