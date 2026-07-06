package permissions

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
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

// SessionTracker tracks user consent decisions for the current session.
// It allows pre-approving patterns so the user isn't prompted repeatedly.
type SessionTracker struct {
	mu       sync.RWMutex
	allowMap map[string]Consent // key: "action:command" -> ConsentAlways
}

// NewSessionTracker creates a new session consent tracker.
func NewSessionTracker() *SessionTracker {
	return &SessionTracker{
		allowMap: make(map[string]Consent),
	}
}

// AddAllow adds a pre-approved command pattern for the session.
// pattern can be a prefix or exact match of the command.
func (st *SessionTracker) AddAllow(action ActionType, command string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	key := st.makeKey(action, command)
	st.allowMap[key] = ConsentAlways
}

// AddDeny adds a command pattern to always deny.
func (st *SessionTracker) AddDeny(action ActionType, command string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	key := st.makeKey(action, command)
	st.allowMap[key] = ConsentDenied
}

// Check checks if a request has been pre-approved or pre-denied.
// Returns (Consent, true) if a matching entry exists, or (ConsentDenied, false) if not.
func (st *SessionTracker) Check(req Request) (Consent, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()

	// Check exact match first
	key := st.makeKey(req.Action, req.Command)
	if c, ok := st.allowMap[key]; ok {
		return c, true
	}

	// Check prefix matches (e.g., "/allow ls" matches "ls -la /home")
	for k, c := range st.allowMap {
		parts := strings.SplitN(k, ":", 2)
		if len(parts) == 2 {
			actionStr := parts[0]
			pattern := parts[1]
			if actionStr == fmt.Sprintf("%d", req.Action) {
				if strings.HasPrefix(req.Command, pattern) {
					return c, true
				}
			}
		}
	}

	return ConsentDenied, false
}

// Clear resets all session-level consents.
func (st *SessionTracker) Clear() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.allowMap = make(map[string]Consent)
}

// List returns all pre-approved patterns.
func (st *SessionTracker) List() []string {
	st.mu.RLock()
	defer st.mu.RUnlock()
	var result []string
	for k, c := range st.allowMap {
		label := "❌"
		if c == ConsentAlways {
			label = "✅"
		}
		parts := strings.SplitN(k, ":", 2)
		if len(parts) == 2 {
			result = append(result, fmt.Sprintf("%s %s %s", label, actionTypeFromString(parts[0]), parts[1]))
		}
	}
	return result
}

func (st *SessionTracker) makeKey(action ActionType, command string) string {
	return fmt.Sprintf("%d:%s", action, command)
}

func actionTypeFromString(s string) string {
	switch s {
	case "0":
		return "shell"
	case "1":
		return "read-file"
	case "2":
		return "write-file"
	case "3":
		return "network"
	case "4":
		return "code-exec"
	default:
		return "unknown"
	}
}

// DefaultSessionTracker is the global session tracker.
var DefaultSessionTracker = NewSessionTracker()

// Prompt displays a permission prompt to the user and returns their decision.
// Checks the session tracker first before prompting.
func Prompt(req Request) (Consent, error) {
	// Check session-level consent first
	if consent, ok := DefaultSessionTracker.Check(req); ok {
		if consent == ConsentAlways {
			fmt.Printf("🔓  [Session] %s: %s (pre-approved via /allow)\n", req.Action, req.Command)
		} else {
			fmt.Printf("🔒  [Session] %s: %s (pre-denied via /deny)\n", req.Action, req.Command)
		}
		return consent, nil
	}

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
	fmt.Print("   Allow? (y/N/a/! for always allow this session): ")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return ConsentDenied, fmt.Errorf("reading input: %w", err)
	}

	input = strings.TrimSpace(strings.ToLower(input))
	switch input {
	case "y", "yes":
		return ConsentAllowed, nil
	case "a", "always", "!":
		// Remember this decision for the session
		DefaultSessionTracker.AddAllow(req.Action, req.Command)
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
