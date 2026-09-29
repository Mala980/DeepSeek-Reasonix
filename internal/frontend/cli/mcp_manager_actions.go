package cli

// mcp_manager_actions.go applies /mcp manager actions: connect, disable, remove,
// mode, auth, and config editing.

import (
	"fmt"
	"os/exec"
	"strings"

	"reasonix/internal/platform/openwith"
)

func mcpOpenCommand(target string) (*exec.Cmd, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("empty target")
	}
	return openwith.URL(target), nil
}
