package permissions

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// ActionType represents the type of system action a model wants to perform.
type ActionType int

const (
	ActionShell ActionType = iota
	ActionFileRead
	ActionFileWrite
	ActionNetwork
	ActionCodeExec
)

func (a ActionType) String() string {
	switch a {
	case ActionShell:
		return "shell command"
	case ActionFileRead:
		return "read file"
	case ActionFileWrite:
		return "write file"
	case ActionNetwork:
		return "network access"
	case ActionCodeExec:
		return "code execution"
	default:
		return "unknown action"
	}
}

// Request represents a permission request from the model.
type Request struct {
	Action      ActionType
	Description string
	Command     string // The actual command, path, URL, etc.
	Context     string // Additional context (why the model says it needs this)
}

// Consent represents the user's decision.
type Consent int

const (
	ConsentDenied  Consent = 0
	ConsentAllowed Consent = 1
	ConsentAlways  Consent = 2 // Allow this action forever (session-level)
)

// Prompt displays a permission prompt to the user and returns their decision.
func Prompt(req Request) (Consent, error) {
	icon := ""
	switch req.Action {
	case ActionShell:
		icon = "💻"
	case ActionFileRead:
		icon = "📖"
	case ActionFileWrite:
		icon = "✏️"
	case ActionNetwork:
		icon = "🌐"
	case ActionCodeExec:
		icon = "⚙️"
	}

	fmt.Printf("\n%s  [Model] wants to perform a %s:\n", icon, req.Action)
	if req.Command != "" {
		fmt.Printf("   Command: %s\n", req.Command)
	}
	if req.Description != "" {
		fmt.Printf("   Details: %s\n", req.Description)
	}
	if req.Context != "" {
		fmt.Printf("   Reason:  %s\n", req.Context)
	}
	fmt.Print("   Allow? (y/N/a for always allow this session): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ConsentDenied, fmt.Errorf("reading input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "y", "yes":
		return ConsentAllowed, nil
	case "a", "always":
		return ConsentAlways, nil
	default:
		return ConsentDenied, nil
	}
}

// AllowAll returns true for all requests — used when running in --no-permissions mode
// or when the user has opted into full autonomy mode.
func AllowAll() bool {
	// Check environment variable for non-interactive mode
	return os.Getenv("UNBOUND_ALLOW_ALL") == "1"
}
