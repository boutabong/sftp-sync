package notify

import (
	"os/exec"
)

// Urgency levels for notifications
type Urgency string

const (
	UrgencyNormal   Urgency = "normal"
	UrgencyCritical Urgency = "critical"
)

// Send displays a desktop notification using notify-send
func Send(title, message string, urgency Urgency) error {
	args := []string{"-u", string(urgency), title, message}
	cmd := exec.Command("notify-send", args...)
	return cmd.Run()
}

// Success sends a success notification
func Success(title, message string) error {
	return Send("✓ "+title, message, UrgencyCritical)
}

// Error sends an error notification
func Error(title, message string) error {
	return Send("✗ "+title, message, UrgencyCritical)
}

// Warning sends a warning notification
func Warning(title, message string) error {
	return Send("⚠ "+title, message, UrgencyNormal)
}

// Info sends an info notification
func Info(title, message string) error {
	return Send(title, message, UrgencyNormal)
}
