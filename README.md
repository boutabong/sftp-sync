# SFTP-Sync

A powerful FTP/SFTP synchronization and mounting tool written in Go.

**Platform:** Linux only (requires FUSE and libnotify)

## Features

- **Bidirectional sync**: Upload and download entire directories
- **Single file operations**: Push/pull individual files
- **Remote mounting**: Mount FTP/SFTP servers as local filesystems
- **Yazi integration**: Browse remote files with yazi file manager in a floating window
- **Multi-profile support**: Manage multiple server configurations
- **Desktop notifications**: Get notified of sync status
- **Smart error handling**: Detailed error messages and recovery

## Installation

### Build from source

```bash
cd sftp-sync-go
go build -o sftp-sync
sudo mv sftp-sync /usr/local/bin/
```

### Dependencies

**Core Requirements (always needed):**
- `lftp` - FTP/SFTP client for sync operations
- `notify-send` - Desktop notifications (libnotify package)

**Protocol-Specific (needed for mounting):**
- `sshfs` - Required for SFTP mounting
- `rclone` - Required for FTP mounting

**Optional (only for --yazi feature):**
- `kitty` - Terminal emulator (required for --yazi flag)
- `yazi` - File manager (required for --yazi flag)

**Installation example (Arch Linux):**
```bash
# Core dependencies
sudo pacman -S lftp libnotify

# For SFTP mounting
sudo pacman -S sshfs

# For FTP mounting
sudo pacman -S rclone

# Optional: for --yazi feature
sudo pacman -S kitty yazi
```

## Configuration

Create config file at `~/.config/sftp-sync/config.json`:

```json
{
  "myserver": {
    "host": "ftp.example.com",
    "username": "user",
    "password": "password",
    "port": 21,
    "protocol": "ftp",
    "remotePath": "/public_html",
    "context": "/home/user/projects/website"
  },
  "webserver": {
    "host": "example.com",
    "username": "user",
    "password": "password",
    "port": 22,
    "protocol": "sftp",
    "remotePath": "/var/www/html",
    "context": "/home/user/projects/webapp"
  },
  "sshkey-server": {
    "host": "example.com",
    "username": "user",
    "sshKey": "/home/user/.ssh/id_rsa",
    "port": 22,
    "protocol": "sftp",
    "remotePath": "/var/www/html",
    "context": "/home/user/projects/webapp"
  }
}
```

### Config Fields

- `host` (required) - Server hostname or IP
- `username` (required) - Login username
- `password` (optional for SFTP with SSH key, required for FTP) - Login password
- `sshKey` (optional) - Path to SSH private key file (for SFTP only)
- `port` (optional) - Port number (default: 21 for FTP, 22 for SFTP)
- `protocol` (optional) - "ftp" or "sftp" (default: "ftp")
- `remotePath` (optional) - Remote directory path (default: "/")
- `context` (required) - Local directory path

**Note for SFTP:** You can use either `password` or `sshKey` for authentication. If both are provided, `sshKey` will be preferred.

## Usage

### Sync Commands

```bash
# Upload entire directory
sftp-sync up myserver

# Download entire directory
sftp-sync down myserver

# Preview what would be uploaded
sftp-sync diff myserver

# Upload single file
sftp-sync push myserver index.html

# Download single file
sftp-sync pull myserver style.css
```

### Mount Commands

```bash
# Mount remote filesystem
sftp-sync mount myserver
# Mounts to: ~/.mounted/myserver/

# Mount and open in yazi
sftp-sync mount myserver --yazi
# Opens yazi in floating kitty window
# Auto-unmounts when yazi closes

# Unmount a profile
sftp-sync unmount myserver

# Unmount all profiles
sftp-sync unmount --all

# List mounted profiles
sftp-sync mounts
```

### Niri Window Rules

Add to your `~/.config/niri/niri.kdl` for floating yazi windows:

```kdl
window-rule {
    match title="^SFTP-Mount-.*"
    default-floating true
    default-width 1400
    default-height 900
}
```

## How It Works

### Sync Operations
- Uses `lftp` for reliable FTP/SFTP synchronization
- Supports mirroring with deletions
- Handles .ftpquota files gracefully
- Counts transferred files
- Detailed error reporting

### Mounting
- **SFTP**: Uses `sshfs` with FUSE
- **FTP**: Uses `rclone` with FUSE
- Verifies remote reachability before mounting
- Prevents duplicate mounts
- Force unmount support for stuck mounts
- Auto-cleanup on exit

### Yazi Integration
- Launches in `kitty` terminal with unique title
- Window title format: `SFTP-Mount-<profile>`
- Blocks until yazi exits
- Auto-unmounts when closed

## License

Original Script Rewritten to Go.
