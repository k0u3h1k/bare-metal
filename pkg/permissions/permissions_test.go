package permissions

import (
	"testing"
)

func TestSessionTracker_AddAllow(t *testing.T) {
	st := NewSessionTracker()
	st.AddAllow(ActionShell, "ls")

	req := Request{Action: ActionShell, Command: "ls -la"}
	consent, ok := st.Check(req)
	if !ok {
		t.Error("expected session tracker to have a match")
	}
	if consent != ConsentAlways {
		t.Errorf("expected ConsentAlways, got %v", consent)
	}
}

func TestSessionTracker_AddDeny(t *testing.T) {
	st := NewSessionTracker()
	st.AddDeny(ActionShell, "rm")

	req := Request{Action: ActionShell, Command: "rm -rf /"}
	consent, ok := st.Check(req)
	if !ok {
		t.Error("expected session tracker to have a match")
	}
	if consent != ConsentDenied {
		t.Errorf("expected ConsentDenied, got %v", consent)
	}
}

func TestSessionTracker_PrefixMatch(t *testing.T) {
	st := NewSessionTracker()
	st.AddAllow(ActionShell, "ls")

	// Should match commands starting with "ls"
	req := Request{Action: ActionShell, Command: "ls -la /home"}
	consent, ok := st.Check(req)
	if !ok {
		t.Error("expected prefix match")
	}
	if consent != ConsentAlways {
		t.Errorf("expected ConsentAlways, got %v", consent)
	}
}

func TestSessionTracker_NoMatch(t *testing.T) {
	st := NewSessionTracker()
	st.AddAllow(ActionShell, "ls")

	req := Request{Action: ActionShell, Command: "cat /etc/passwd"}
	_, ok := st.Check(req)
	if ok {
		t.Error("expected no match for different command")
	}
}

func TestSessionTracker_Clear(t *testing.T) {
	st := NewSessionTracker()
	st.AddAllow(ActionShell, "ls")
	st.Clear()

	req := Request{Action: ActionShell, Command: "ls -la"}
	_, ok := st.Check(req)
	if ok {
		t.Error("expected no match after clear")
	}
}

func TestSessionTracker_ActionTypeMismatch(t *testing.T) {
	st := NewSessionTracker()
	st.AddAllow(ActionShell, "ls")

	// Same command but different action type should not match
	req := Request{Action: ActionFileRead, Command: "ls"}
	_, ok := st.Check(req)
	if ok {
		t.Error("expected no match for different action type")
	}
}

func TestSessionTracker_MultiplePatterns(t *testing.T) {
	st := NewSessionTracker()
	st.AddAllow(ActionShell, "ls")
	st.AddDeny(ActionShell, "rm")
	st.AddAllow(ActionFileRead, "/home")

	// Should match allow
	req1 := Request{Action: ActionShell, Command: "ls -la"}
	c1, ok1 := st.Check(req1)
	if !ok1 || c1 != ConsentAlways {
		t.Error("expected allow match for ls")
	}

	// Should match deny
	req2 := Request{Action: ActionShell, Command: "rm file.txt"}
	c2, ok2 := st.Check(req2)
	if !ok2 || c2 != ConsentDenied {
		t.Error("expected deny match for rm")
	}

	// Should match file read
	req3 := Request{Action: ActionFileRead, Command: "/home/user/file.txt"}
	c3, ok3 := st.Check(req3)
	if !ok3 || c3 != ConsentAlways {
		t.Error("expected allow match for file read")
	}

	// Unknown command should not match
	req4 := Request{Action: ActionShell, Command: "curl http://example.com"}
	_, ok4 := st.Check(req4)
	if ok4 {
		t.Error("expected no match for unknown command")
	}
}
