package run

import (
	"fmt"

	"github.com/k0u3h1k/bare-metal/pkg/console"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound run` command.
var Cmd = &cobra.Command{
	Use:   "run <model-name>",
	Short: "Download (if needed) and start an interactive chat with a model",
	Long: `Downloads a model from Hugging Face if not already cached,
then starts an interactive terminal chat session. Models can be granted
full system permissions (shell, file I/O, internet) with explicit consent.

Example:
  unbound run llama3.2:1b
  unbound run mistral:7b
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		fmt.Printf("🔧 Unbound — running model: %s\n\n", modelName)

		// TODO: resolve model name, download if needed, load into llama.cpp
		fmt.Println("📥 Model resolution and loading coming soon...")
		fmt.Println("💬 Starting interactive chat (placeholder)")

		return console.RunInteractive(modelName)
	},
}

func init() {
	Cmd.Flags().BoolP("no-permissions", "n", false, "Run in sandboxed mode (no permissions)")
	Cmd.Flags().StringP("system-prompt", "s", "", "Custom system prompt for the model")
	Cmd.Flags().IntP("max-tokens", "m", 2048, "Maximum tokens per response")
}
