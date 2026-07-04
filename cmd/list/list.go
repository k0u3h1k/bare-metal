package list

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0u3h1k/bare-metal/internal/config"
	"github.com/spf13/cobra"
)

// Cmd represents the `unbound list` command.
var Cmd = &cobra.Command{
	Use:   "list",
	Short: "List locally cached models",
	Long: `Lists all models that have been downloaded and cached locally.
Models are stored in ~/.unbound/models/ by default.

Example:
  unbound list
`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		modelDir := config.App.ModelDir

		entries, err := os.ReadDir(modelDir)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Println("No models cached yet. Use 'unbound pull <model>' to download one.")
				return nil
			}
			return fmt.Errorf("reading model directory: %w", err)
		}

		if len(entries) == 0 {
			fmt.Println("No models cached yet. Use 'unbound pull <model>' to download one.")
			return nil
		}

		fmt.Println("📦 Cached models:")
		for _, e := range entries {
			if !e.IsDir() {
				info, _ := e.Info()
				sizeMB := float64(info.Size()) / (1024 * 1024)
				fmt.Printf("  • %s (%.1f MB)\n", e.Name(), sizeMB)
				continue
			}
			// Show directory contents for subdirectories
			subEntries, _ := os.ReadDir(filepath.Join(modelDir, e.Name()))
			for _, s := range subEntries {
				info, _ := s.Info()
				if info != nil && !info.IsDir() {
					sizeMB := float64(info.Size()) / (1024 * 1024)
					fmt.Printf("  • %s/%s (%.1f MB)\n", e.Name(), s.Name(), sizeMB)
				}
			}
		}
		return nil
	},
}
