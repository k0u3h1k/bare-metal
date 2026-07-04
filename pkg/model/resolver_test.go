package model

import (
	"testing"
)

func TestResolveModel_Alias(t *testing.T) {
	tests := []struct {
		input    string
		wantRepo string
		wantFile string
		wantErr  bool
	}{
		{"llama3.2:1b", "hugging-quants/Llama-3.2-1B-Instruct-Q4_K_M-GGUF", "Llama-3.2-1B-Instruct-Q4_K_M.gguf", false},
		{"mistral:7b", "MaziyarPanahi/Mistral-7B-Instruct-v0.3-GGUF", "Mistral-7B-Instruct-v0.3.Q4_K_M.gguf", false},
		{"llama3.2:3b", "hugging-quants/Llama-3.2-3B-Instruct-Q4_K_M-GGUF", "Llama-3.2-3B-Instruct-Q4_K_M.gguf", false},
		{"qwen2:7b", "hugging-quants/Qwen2.5-7B-Instruct-Q4_K_M-GGUF", "Qwen2.5-7B-Instruct-Q4_K_M.gguf", false},
		{"unknown:model", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ResolveModel(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ResolveModel(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("ResolveModel(%q) unexpected error: %v", tt.input, err)
				return
			}
			if result.RepoID != tt.wantRepo {
				t.Errorf("ResolveModel(%q) repo = %q, want %q", tt.input, result.RepoID, tt.wantRepo)
			}
			if result.Filename != tt.wantFile {
				t.Errorf("ResolveModel(%q) file = %q, want %q", tt.input, result.Filename, tt.wantFile)
			}
			if !result.IsAlias {
				t.Errorf("ResolveModel(%q) IsAlias = false, want true", tt.input)
			}
		})
	}
}

func TestResolveModel_FullRepo(t *testing.T) {
	result, err := ResolveModel("meta-llama/Llama-3.2-1B-Instruct-GGUF")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.RepoID != "meta-llama/Llama-3.2-1B-Instruct-GGUF" {
		t.Errorf("repo = %q, want %q", result.RepoID, "meta-llama/Llama-3.2-1B-Instruct-GGUF")
	}
	if result.IsAlias {
		t.Errorf("IsAlias should be false for full repo IDs")
	}
}

func TestResolveModel_RepoWithFile(t *testing.T) {
	result, err := ResolveModel("org/repo:my-model.gguf")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.RepoID != "org/repo" {
		t.Errorf("repo = %q, want %q", result.RepoID, "org/repo")
	}
	if result.Filename != "my-model.gguf" {
		t.Errorf("file = %q, want %q", result.Filename, "my-model.gguf")
	}
}

func TestResolveModel_Unknown(t *testing.T) {
	_, err := ResolveModel("totally-fake-model-name")
	if err == nil {
		t.Error("expected error for unknown model, got nil")
	}
}

func TestListBuiltinAliases(t *testing.T) {
	aliases := ListBuiltinAliases()
	if len(aliases) == 0 {
		t.Error("expected at least one built-in alias")
	}
	// Check for a few expected aliases
	found := false
	for _, a := range aliases {
		if a == "llama3.2:1b" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'llama3.2:1b' in built-in aliases")
	}
}
