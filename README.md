# SFTP-Sync

A fast, powerful FTP/SFTP synchronization and mounting tool written in Go. Think of it as rsync for remote servers, but with auto-sync capabilities, desktop notifications, and filesystem mounting built in.

**Platform:** Linux only (requires FUSE and libnotify)
**Version:** 2.3.1

## Why SFTP-Sync?

- **Bidirectional sync** - Upload/download entire directories with one command
- **Auto-sync daemon** - Watches your files and uploads changes automatically
- **Remote mounting** - Browse remote servers like local folders
- **Editor integration** - Smart project detection works seamlessly with any editor
- **Desktop notifications** - Never wonder if your sync worked
- **Multi-profile** - Manage multiple servers effortlessly
- **.syncignore support** - Exclude files from auto-sync (like .gitignore)
- **Yazi integration** - Browse remote files in a floating window

## Installation

### Build from Source

```bash
cd sftp-sync
go build -o sftp-sync
sudo mv sftp-sync /usr/local/bin/
```

### Dependencies

**Always Required:**
- `lftp` - The actual sync engine
- `notify-send` - Desktop notifications (from libnotify)

**For Mounting:**
- `sshfs` - SFTP mounting
- `rclone` - FTP mounting

**Optional (for --yazi feature):**
- `kitty` - Terminal emulator
- `yazi` - File manager

**Install on Arch Linux:**
```bash
# Core
sudo pacman -S lftp libnotify

# For mounting
sudo pacman -S sshfs rclone

# Optional: for --yazi
sudo pacman -S kitty yazi
```

## Configuration

Config lives at `~/.config/sftp-sync/config.json`

### Basic Example

```json
{
  "myserver": {
    "host": "example.com",
    "username": "user",
    "password": "password",
    "port": 22,
    "protocol": "sftp",
    "remotePath": "/var/www/html"
  }
}
```

### SSH Key Authentication (Recommended)

```json
{
  "webserver": {
    "host": "example.com",
    "username": "user",
    "sshKey": "/home/user/.ssh/id_rsa",
    "port": 22,
    "protocol": "sftp",
    "remotePath": "/var/www/html"
  }
}
```

### Auto-Sync Daemon Configuration

Want files to upload automatically when you save them? Add `autoSync: true`:

```json
{
  "myproject": {
    "host": "example.com",
    "username": "user",
    "sshKey": "/home/user/.ssh/id_rsa",
    "protocol": "sftp",
    "remotePath": "/var/www/html",
    "context": "/home/user/projects/myproject",
    "autoSync": true,
    "autoSyncDebounce": 2000
  }
}
```

### All Configuration Options

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `host` | **Yes** | - | Server hostname or IP address |
| `username` | **Yes** | - | Login username |
| `password` | No* | - | Login password (see security note below) |
| `sshKey` | No* | - | Path to SSH private key (SFTP only) |
| `port` | No | 21 (FTP)<br>22 (SFTP) | Port number |
| `protocol` | No | `"ftp"` | `"ftp"` or `"sftp"` |
| `remotePath` | No | `"/"` | Remote directory path |
| `context` | No | `~/.mounted/<profile>` | Mount point directory |
| `autoSync` | No | `false` | Enable auto-sync daemon for this profile |
| `autoSyncDebounce` | No | `2000` | Milliseconds to wait before uploading (prevents thrashing) |

*Either `password` or `sshKey` required. SSH key preferred for SFTP.

### Security Warning

**Passwords are stored in plaintext** in your config file. This is a limitation of how lftp works.

**Important:** Protect your config file:
```bash
chmod 600 ~/.config/sftp-sync/config.json
```

For better security, use SSH key authentication instead of passwords when possible.

## Usage

### Basic Sync Commands

```bash
# Upload current directory to remote
cd /path/to/project
sftp-sync up myserver

# Download from remote to current directory
sftp-sync down myserver

# Preview what would be uploaded (dry-run)
sftp-sync diff myserver
```

### Single File Operations

```bash
# Upload one file
sftp-sync push myserver index.html

# Download one file
sftp-sync pull myserver style.css

# Upload current file (detects project root automatically)
sftp-sync current myserver /home/user/project/src/main.go
```

### Mounting

```bash
# Mount remote filesystem
sftp-sync mount myserver
# Default: ~/.mounted/myserver/

# Mount and browse with yazi
sftp-sync mount myserver --yazi
# Opens in floating kitty window, auto-unmounts on exit

# Unmount
sftp-sync unmount myserver

# Unmount everything
sftp-sync unmount --all

# List mounted profiles
sftp-sync mounts
```

### Auto-Sync Daemon

The daemon watches your files and uploads changes automatically.

```bash
# Run daemon in foreground (for testing)
sftp-sync daemon

# Install as systemd user service
sftp-sync install-daemon

# Start the service
systemctl --user start sftp-sync
systemctl --user enable sftp-sync  # Start on boot

# Check status
systemctl --user status sftp-sync

# View logs
journalctl --user -u sftp-sync -f

# Uninstall
sftp-sync uninstall-daemon
```

**How it works:**
- Watches all directories configured with `"autoSync": true`
- Debounces file changes (waits 2s by default before uploading)
- Batches notifications (won't spam you)
- Retries failed uploads with exponential backoff
- Hot-reloads config when you edit it
- Respects `.syncignore` patterns

### .syncignore File

Create a `.syncignore` file in your project root to exclude files from auto-sync:

```gitignore
# Ignore patterns (like .gitignore syntax)
*.log
*.tmp
node_modules/
.git/
dist/
.env

# Use doublestar patterns
**/*.backup
**/temp/**
```

**Pattern syntax:**
- Uses [doublestar](https://github.com/bmatcuk/doublestar) glob patterns
- `*` matches anything except `/`
- `**` matches anything including `/`
- `?` matches single character
- Relative to project root

## Editor Integration

sftp-sync detects project roots automatically when called with absolute paths. Perfect for editor keybindings!

### Helix

Add to `~/.config/helix/config.toml`:

```toml
[keys.normal.space]
u = ":run-shell-command sftp-sync up myserver %{buffer_name}"
d = ":run-shell-command sftp-sync down myserver %{buffer_name}"
c = ":run-shell-command sftp-sync current myserver %{buffer_name}"
```

**How it works:**
- Editor passes absolute path like `/home/user/project/src/file.js`
- sftp-sync walks up directories to find `.git` (project root)
- Uses project root as sync context
- Works from anywhere in your project

### Other Editors

Any editor that can run shell commands with the current file path will work. Just pass the absolute path as the last argument.

## Window Manager Integration

### Niri

Floating yazi windows with `--yazi` flag:

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
- Uses `lftp` mirror command for reliable sync
- Supports deletions (mirror mode)
- Automatically skips `.ftpquota` files
- Counts transferred files
- Shows detailed errors

### Mounting
- **SFTP:** Uses `sshfs` with FUSE
- **FTP:** Uses `rclone` with FUSE
- Checks server reachability before mounting
- Prevents duplicate mounts
- Force unmount support
- Auto-cleanup on exit

### Auto-Sync Daemon
- File watching with `fsnotify`
- Debouncing prevents rapid re-uploads during saves
- Upload queue with retry logic (1s, 2s, 4s backoff)
- Notification batching (shows summary every 30s or 5 files)
- Config hot-reload (edit config.json while daemon runs)
- Multiple profile support with context specificity matching

### Context Detection

When you sync from a subdirectory, sftp-sync finds your project root by walking up and looking for `.git`. This lets you run sync commands from anywhere in your project.

For overlapping contexts (e.g., one project inside another), sftp-sync picks the most specific match.

## Troubleshooting

### "Profile not found"
Check your config file exists at `~/.config/sftp-sync/config.json` and has valid JSON syntax.

### "Permission denied" on mount
Make sure the mount point directory exists and you have write permissions.

### Auto-sync not working
1. Check `systemctl --user status sftp-sync`
2. View logs: `journalctl --user -u sftp-sync -f`
3. Verify `"autoSync": true` in config
4. Make sure `context` path exists and matches your project

### Notifications not showing
- Check `notify-send` is installed
- Some desktop environments filter notifications by urgency
- Test: `notify-send "Test" "Message"`

### Mount stuck / won't unmount
```bash
# Force unmount
fusermount -u ~/.mounted/profilename

# Or unmount all with force flag
sftp-sync unmount --all
```

### IPv6 addresses
IPv6 is fully supported. Just use the address as-is:
```json
{
  "host": "2001:db8::1"
}
```

### SSH key not working
- Make sure key file has correct permissions: `chmod 600 ~/.ssh/id_rsa`
- Verify key is in the correct format (OpenSSH or PEM)
- Test manually: `ssh -i ~/.ssh/id_rsa user@host`

## Performance Tips

### Large Directories
- Use `.syncignore` to exclude `node_modules`, build artifacts, etc.
- Consider syncing subdirectories individually
- Use `diff` command first to preview changes

### Many Small Files
- Auto-sync daemon queues uploads (max 100 pending)
- Adjust `autoSyncDebounce` if saves are too frequent
- Consider batch syncing with `up` command instead

## Project History

Originally a Fish shell script, rewritten in Go for better performance, reliability, and features.

## License

Free to use and modify.
