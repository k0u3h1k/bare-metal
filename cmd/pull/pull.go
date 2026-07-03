package pull

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Cmd represents the `unbound pull` command.
var Cmd = &cobra.Command{
	Use:   "pull <model-name>",
	Short: "Download a model from Hugging Face",
	Long: `Downloads a model from Hugging Face Hub and caches it locally.
Models are stored in ~/.unbound/models/ by default.

Supported formats:
  - Hugging Face repo IDs: "meta-llama/Llama-3.2-1B"
  - Short aliases: "llama3.2:1b", "mistral:7b", "codellama:13b"

Example:
  unbound pull llama3.2:1b
  unbound pull meta-llama/Llama-3.2-1B-Instruct-GGUF
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]

		fmt.Printf("📥 Pulling model: %s\n", modelName)
		fmt.Println("   (Model download not yet implemented — this is a placeholder)")
		fmt.Println("   Future: downloads GGUF files from Hugging Face Hub")
		return nil
	},
}
