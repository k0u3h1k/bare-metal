package remove

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0u3h1k/bare-metal/internal/config"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound remove` command.
var Cmd = &cobra.Command{
	Use:   "remove <model-name>",
	Short: "Remove a cached model",
	Long: `Removes a previously downloaded model from the local cache.

Example:
  unbound remove llama3.2:1b
  unbound remove meta-llama/Llama-3.2-1B-Instruct-GGUF
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]
		modelPath := filepath.Join(config.App.ModelDir, modelName)

		// Check if model exists as a file or directory
		_, err := os.Stat(modelPath)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("model '%s' not found in cache", modelName)
			}
			return fmt.Errorf("accessing model: %w", err)
		}

		if err := os.RemoveAll(modelPath); err != nil {
			return fmt.Errorf("removing model: %w", err)
		}

		fmt.Printf("🗑️  Removed model: %s\n", modelName)
		return nil
	},
}
