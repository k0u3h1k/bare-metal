package list

import (
	"fmt"

	"github.com/k0u3h1k/bare-metal/pkg/model"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound list` command.
var Cmd = &cobra.Command{
	Use:   "list",
	Short: "List locally cached models",
	Long: `Lists all models that have been downloaded and cached locally.
Models are stored in ~/.unbound/models/ by default.
Shows model name, size, parameters, and quantization when available.

Example:
  unbound list
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		mgr := model.NewManager()
		models, err := mgr.List()
		if err != nil {
			return fmt.Errorf("listing models: %w", err)
		}

		if len(models) == 0 {
			fmt.Println("No models cached yet. Use 'unbound pull <model>' to download one.")
			fmt.Println("\nBuilt-in aliases available: llama3.2:1b, llama3.2:3b, mistral:7b, qwen2:7b, phi3:3.8b, gemma2:2b, and more.")
			return nil
		}

		fmt.Println("📦 Cached models:")
		for _, m := range models {
			status := "✅"
			if !m.IsReady {
				status = "⏳"
			}
			details := m.SizeHuman
			if m.Params != "" {
				details = fmt.Sprintf("%s  |  %s params  |  %s", m.SizeHuman, m.Params, m.Quantization)
			}
			fmt.Printf("  %s  %-20s %s\n", status, m.Name, details)
		}
		return nil
	},
}
