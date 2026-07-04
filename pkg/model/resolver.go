package model

import (
	"fmt"
	"strings"
)

// ModelAlias maps a short user-friendly name to a Hugging Face repo ID.
type ModelAlias struct {
	RepoID       string // Hugging Face repo ID
	DefaultFile  string // Default GGUF filename pattern (empty means auto-detect)
	Params       string // Parameter count e.g. "1B", "7B"
	Quantization string // Default quantization
}

// builtinAliases is the registry of well-known model aliases.
// Users can also specify full HF repo IDs directly.
var builtinAliases = map[string]ModelAlias{
	// Llama 3.2 (Meta)
	"llama3.2:1b": {RepoID: "hugging-quants/Llama-3.2-1B-Instruct-Q4_K_M-GGUF", DefaultFile: "Llama-3.2-1B-Instruct-Q4_K_M.gguf", Params: "1B", Quantization: "Q4_K_M"},
	"llama3.2:3b": {RepoID: "hugging-quants/Llama-3.2-3B-Instruct-Q4_K_M-GGUF", DefaultFile: "Llama-3.2-3B-Instruct-Q4_K_M.gguf", Params: "3B", Quantization: "Q4_K_M"},

	// Llama 3.1 (Meta)
	"llama3.1:8b": {RepoID: "hugging-quants/Meta-Llama-3.1-8B-Instruct-Q4_K_M-GGUF", DefaultFile: "Meta-Llama-3.1-8B-Instruct-Q4_K_M.gguf", Params: "8B", Quantization: "Q4_K_M"},

	// Mistral
	"mistral:7b":   {RepoID: "MaziyarPanahi/Mistral-7B-Instruct-v0.3-GGUF", DefaultFile: "Mistral-7B-Instruct-v0.3.Q4_K_M.gguf", Params: "7B", Quantization: "Q4_K_M"},
	"mistral:nemo": {RepoID: "hugging-quants/Mistral-Nemo-Instruct-2407-Q4_K_M-GGUF", DefaultFile: "mistral-nemo-instruct-2407.Q4_K_M.gguf", Params: "12B", Quantization: "Q4_K_M"},

	// CodeLlama
	"codellama:7b":  {RepoID: "hugging-quants/CodeLlama-7b-Instruct-hf-Q4_K_M-GGUF", DefaultFile: "CodeLlama-7b-Instruct-hf.Q4_K_M.gguf", Params: "7B", Quantization: "Q4_K_M"},
	"codellama:13b": {RepoID: "hugging-quants/CodeLlama-13b-Instruct-hf-Q4_K_M-GGUF", DefaultFile: "CodeLlama-13b-Instruct-hf.Q4_K_M.gguf", Params: "13B", Quantization: "Q4_K_M"},

	// DeepSeek
	"deepseek:67b": {RepoID: "hugging-quants/deepseek-llm-67b-chat-Q4_K_M-GGUF", DefaultFile: "deepseek-llm-67b-chat.Q4_K_M.gguf", Params: "67B", Quantization: "Q4_K_M"},

	// Qwen
	"qwen2:0.5b": {RepoID: "hugging-quants/Qwen2.5-0.5B-Instruct-Q4_K_M-GGUF", DefaultFile: "Qwen2.5-0.5B-Instruct-Q4_K_M.gguf", Params: "0.5B", Quantization: "Q4_K_M"},
	"qwen2:1.5b": {RepoID: "hugging-quants/Qwen2.5-1.5B-Instruct-Q4_K_M-GGUF", DefaultFile: "Qwen2.5-1.5B-Instruct-Q4_K_M.gguf", Params: "1.5B", Quantization: "Q4_K_M"},
	"qwen2:7b":   {RepoID: "hugging-quants/Qwen2.5-7B-Instruct-Q4_K_M-GGUF", DefaultFile: "Qwen2.5-7B-Instruct-Q4_K_M.gguf", Params: "7B", Quantization: "Q4_K_M"},

	// Phi-3 (Microsoft)
	"phi3:3.8b": {RepoID: "microsoft/Phi-3-mini-4k-instruct-gguf", DefaultFile: "Phi-3-mini-4k-instruct.Q4_K_M.gguf", Params: "3.8B", Quantization: "Q4_K_M"},

	// Gemma 2
	"gemma2:2b": {RepoID: "hugging-quants/gemma-2-2b-it-Q4_K_M-GGUF", DefaultFile: "gemma-2-2b-it-Q4_K_M.gguf", Params: "2B", Quantization: "Q4_K_M"},
	"gemma2:9b": {RepoID: "hugging-quants/gemma-2-9b-it-Q4_K_M-GGUF", DefaultFile: "gemma-2-9b-it-Q4_K_M.gguf", Params: "9B", Quantization: "Q4_K_M"},

	// Granite (IBM)
	"granite:3b": {RepoID: "hugging-quants/Granite-3.0-3B-Instruct-Q4_K_M-GGUF", DefaultFile: "Granite-3.0-3B-Instruct-Q4_K_M.gguf", Params: "3B", Quantization: "Q4_K_M"},
	"granite:8b": {RepoID: "hugging-quants/Granite-3.0-8B-Instruct-Q4_K_M-GGUF", DefaultFile: "Granite-3.0-8B-Instruct-Q4_K_M.gguf", Params: "8B", Quantization: "Q4_K_M"},

	// Hermes (NousResearch)
	"hermes:8b": {RepoID: "hugging-quants/Hermes-3-Llama-3.1-8B-Q4_K_M-GGUF", DefaultFile: "Hermes-3-Llama-3.1-8B.Q4_K_M.gguf", Params: "8B", Quantization: "Q4_K_M"},
}

// ResolveResult contains the resolved model information.
type ResolveResult struct {
	Alias        string // The original user input
	RepoID       string // Hugging Face repo
	Filename     string // Specific filename to download
	Params       string
	Quantization string
	IsAlias      bool // True if resolved from a built-in alias
}

// ResolveModel takes a user input and resolves it to a Hugging Face repo + file.
// Supports:
//   - Built-in aliases: "llama3.2:1b", "mistral:7b", etc.
//   - Full HF repo IDs: "meta-llama/Llama-3.2-1B-Instruct-GGUF"
//   - Repo+file: "org/repo:filename.gguf"
func ResolveModel(input string) (*ResolveResult, error) {
	// Check built-in aliases first
	if alias, ok := builtinAliases[input]; ok {
		return &ResolveResult{
			Alias:        input,
			RepoID:       alias.RepoID,
			Filename:     alias.DefaultFile,
			Params:       alias.Params,
			Quantization: alias.Quantization,
			IsAlias:      true,
		}, nil
	}

	// If it contains a colon after a slash, it's "org/repo:filename"
	if strings.Contains(input, "/") && strings.Count(input, ":") == 1 {
		parts := strings.SplitN(input, ":", 2)
		return &ResolveResult{
			Alias:    input,
			RepoID:   parts[0],
			Filename: parts[1],
			IsAlias:  false,
		}, nil
	}

	// If it contains a slash, treat as a full repo ID (auto-detect file later)
	if strings.Contains(input, "/") {
		return &ResolveResult{
			Alias:   input,
			RepoID:  input,
			IsAlias: false,
		}, nil
	}

	return nil, fmt.Errorf("unknown model '%s'. Try a built-in alias (e.g. llama3.2:1b, mistral:7b) or a full Hugging Face repo ID", input)
}

// ListBuiltinAliases returns all registered aliases with their descriptions.
func ListBuiltinAliases() []string {
	var names []string
	for name := range builtinAliases {
		names = append(names, name)
	}
	return names
}
