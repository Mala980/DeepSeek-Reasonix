//go:build linux

package notify

import (
	"os/exec"

	"reasonix/internal/secrets"
	"reasonix/internal/termux"
)

// PlatformSender delivers notifications through the host OS.
type PlatformSender struct{}

// NewPlatformSender returns the best-effort sender for the current platform.
func NewPlatformSender() PlatformSender { return PlatformSender{} }

func (PlatformSender) Send(m Message) error {
	cmd := exec.Command("notify-send", m.Title, m.Body)
	cmd.Env = secrets.ProcessEnv()
	if err := cmd.Start(); err != nil {
		if termux.IsAndroidOrTermux() {
			tCmd := exec.Command("termux-notification", "--title", m.Title, "--content", m.Body)
			tCmd.Env = secrets.ProcessEnv()
			if tErr := tCmd.Start(); tErr == nil {
				go func() { _ = tCmd.Wait() }()
				return nil
			}
		}
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
