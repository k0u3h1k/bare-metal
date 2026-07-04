package pull

import (
	"fmt"
	"os"

	"github.com/k0u3h1k/bare-metal/pkg/model"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound pull` command.
var Cmd = &cobra.Command{
	Use:   "pull <model-name>",
	Short: "Download a model from Hugging Face",
	Long: `Downloads a model from Hugging Face Hub and caches it locally.
Models are stored in ~/.unbound/models/ by default.

Supported formats:
  - Built-in aliases: "llama3.2:1b", "mistral:7b", "qwen2:7b", etc.
  - Full Hugging Face repo IDs: "meta-llama/Llama-3.2-1B-Instruct-GGUF"
  - Repo + filename: "org/repo:filename.gguf"

Examples:
  unbound pull llama3.2:1b
  unbound pull mistral:7b
  unbound pull hugging-quants/Meta-Llama-3.1-8B-Instruct-Q4_K_M-GGUF
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]

		mgr := model.NewManager()
		if err := mgr.Pull(modelName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		return nil
	},
}
