package remove

import (
	"fmt"
	"os"

	"github.com/k0u3h1k/bare-metal/pkg/model"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound remove` command.
var Cmd = &cobra.Command{
	Use:   "remove <model-name>",
	Short: "Remove a cached model",
	Long: `Removes a previously downloaded model from the local cache.
Uses the model name as shown by 'unbound list'.

Examples:
  unbound remove llama3.2:1b
  unbound remove hugging-quants/Meta-Llama-3.1-8B-Instruct-Q4_K_M-GGUF
`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		modelName := args[0]

		mgr := model.NewManager()
		if err := mgr.Remove(modelName); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return err
		}
		fmt.Printf("🗑️  Removed model: %s\n", modelName)
		return nil
	},
}
