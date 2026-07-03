package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/k0u3h1k/bare-metal/pkg/permissions"
)

// Result holds the output of a shell execution.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Exec runs a shell command after getting user consent.
func Exec(command string, context string, autoApprove bool) (*Result, error) {
	// If auto-approve is set (e.g., via --no-permissions or env var), skip the prompt
	if !autoApprove && !permissions.AllowAll() {
		consent, err := permissions.Prompt(permissions.Request{
			Action:      permissions.ActionShell,
			Description: fmt.Sprintf("Execute shell command: %s", command),
			Command:     command,
			Context:     context,
		})
		if err != nil {
			return nil, fmt.Errorf("permission prompt error: %w", err)
		}
		if consent == permissions.ConsentDenied {
			return &Result{
				Stderr:   "Permission denied by user",
				ExitCode: 1,
			}, nil
		}
	}

	// Execute the command
	cmd := exec.Command("bash", "-c", command)
	cmd.Stdin = os.Stdin

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	exitCode := 0
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("execution error: %w", err)
		}
	}

	return &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
	}, nil
}

// ReadFile reads a file after getting user consent.
func ReadFile(path string, context string, autoApprove bool) (string, error) {
	if !autoApprove && !permissions.AllowAll() {
		consent, err := permissions.Prompt(permissions.Request{
			Action:      permissions.ActionFileRead,
			Description: fmt.Sprintf("Read file: %s", path),
			Command:     path,
			Context:     context,
		})
		if err != nil {
			return "", fmt.Errorf("permission prompt error: %w", err)
		}
		if consent == permissions.ConsentDenied {
			return "", fmt.Errorf("permission denied by user")
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading file: %w", err)
	}
	return string(data), nil
}

// WriteFile writes content to a file after getting user consent.
func WriteFile(path string, content string, context string, autoApprove bool) error {
	if !autoApprove && !permissions.AllowAll() {
		consent, err := permissions.Prompt(permissions.Request{
			Action:      permissions.ActionFileWrite,
			Description: fmt.Sprintf("Write to file: %s (%d bytes)", path, len(content)),
			Command:     path,
			Context:     context,
		})
		if err != nil {
			return fmt.Errorf("permission prompt error: %w", err)
		}
		if consent == permissions.ConsentDenied {
			return fmt.Errorf("permission denied by user")
		}
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("writing file: %w", err)
	}
	return nil
}

// NetworkAccess checks with the user before allowing outbound network access.
// Returns true if permitted.
func NetworkAccess(url string, context string, autoApprove bool) bool {
	if autoApprove || permissions.AllowAll() {
		return true
	}

	consent, err := permissions.Prompt(permissions.Request{
		Action:      permissions.ActionNetwork,
		Description: fmt.Sprintf("Network access to: %s", url),
		Command:     url,
		Context:     context,
	})
	if err != nil {
		return false
	}
	return consent != permissions.ConsentDenied
}
